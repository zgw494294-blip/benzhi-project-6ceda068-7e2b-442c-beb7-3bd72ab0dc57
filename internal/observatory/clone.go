package observatory

func CloneAggregate(source Aggregate) Aggregate {
	clone := source
	clone.Revisions = make(map[string]DatasetRevision, len(source.Revisions))
	for id, revision := range source.Revisions {
		clone.Revisions[id] = revision
	}
	clone.Findings = make(map[string]ValidationFinding, len(source.Findings))
	for id, finding := range source.Findings {
		if finding.ReviewedAt != nil {
			stamp := *finding.ReviewedAt
			finding.ReviewedAt = &stamp
		}
		clone.Findings[id] = finding
	}
	if source.Manifest != nil {
		manifest := *source.Manifest
		manifest.Entries = append([]ManifestEntry(nil), source.Manifest.Entries...)
		clone.Manifest = &manifest
	}
	if source.Credential != nil {
		credential := *source.Credential
		clone.Credential = &credential
	}
	return clone
}
