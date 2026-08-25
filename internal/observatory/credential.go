package observatory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func IssueCredential(aggregate Aggregate, id, approver, purpose string, now time.Time) (ReleaseCredential, error) {
	if err := EnsureState(aggregate.Task, StateFrozen); err != nil {
		return ReleaseCredential{}, err
	}
	if aggregate.Manifest == nil {
		return ReleaseCredential{}, invalid("MANIFEST_NOT_FOUND", "任务没有冻结清单")
	}
	if reasons := VerifyManifest(*aggregate.Manifest); len(reasons) > 0 {
		return ReleaseCredential{}, invalid("MANIFEST_INVALID", "冻结清单校验失败：%s", strings.Join(reasons, "；"))
	}
	approver = strings.TrimSpace(approver)
	purpose = strings.TrimSpace(purpose)
	if approver == "" {
		return ReleaseCredential{}, invalid("INVALID_APPROVER", "释放批准人不能为空")
	}
	if len([]rune(purpose)) < 4 || len([]rune(purpose)) > 500 {
		return ReleaseCredential{}, invalid("INVALID_PURPOSE_SCOPE", "用途范围长度必须为 4 到 500 个字符")
	}
	credential := ReleaseCredential{
		ID: id, TaskID: aggregate.Task.ID, ManifestID: aggregate.Manifest.ID,
		ManifestRoot: aggregate.Manifest.MerkleRoot, ApprovedBy: approver,
		PurposeScope: purpose, IssuedAt: now.UTC(),
	}
	credential.CredentialDigest = CredentialDigest(credential)
	return credential, nil
}

func CredentialDigest(credential ReleaseCredential) string {
	payload := fmt.Sprintf("release-credential-v1\n%s\n%s\n%s\n%s\n%s\n%s",
		credential.ID, credential.TaskID, credential.ManifestID, credential.ManifestRoot,
		credential.ApprovedBy, credential.PurposeScope+"\n"+credential.IssuedAt.UTC().Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

type VerificationResult struct {
	Valid   bool     `json:"valid"`
	Reasons []string `json:"reasons"`
}

func VerifyRelease(aggregate Aggregate) VerificationResult {
	reasons := make([]string, 0)
	if aggregate.Task.State != StateReleased {
		reasons = append(reasons, "任务状态不是 RELEASED")
	}
	if aggregate.Manifest == nil {
		reasons = append(reasons, "缺少冻结清单")
	}
	if aggregate.Credential == nil {
		reasons = append(reasons, "缺少释放凭据")
	}
	if aggregate.Manifest != nil {
		reasons = append(reasons, VerifyManifest(*aggregate.Manifest)...)
		if aggregate.Manifest.TaskID != aggregate.Task.ID {
			reasons = append(reasons, "清单关联任务不匹配")
		}
	}
	if aggregate.Credential != nil {
		credential := aggregate.Credential
		if CredentialDigest(*credential) != credential.CredentialDigest {
			reasons = append(reasons, "凭据摘要与凭据内容不匹配")
		}
		if credential.TaskID != aggregate.Task.ID {
			reasons = append(reasons, "凭据关联任务不匹配")
		}
		if aggregate.Manifest != nil {
			if credential.ManifestID != aggregate.Manifest.ID || credential.ManifestRoot != aggregate.Manifest.MerkleRoot {
				reasons = append(reasons, "凭据绑定的清单标识或 Merkle 根不匹配")
			}
		}
	}
	return VerificationResult{Valid: len(reasons) == 0, Reasons: reasons}
}
