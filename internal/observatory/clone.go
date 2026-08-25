package observatory

func CloneAggregate(source Aggregate) Aggregate {
	clone := source
	clone.Revisions = make(map[string]DatasetRevision, len(source.Revisions))
	for id, revision := range source.Revisions {
		clone.Revisions[id] = revision
	}
	clone.Findings = make(map[string]ValidationFinding, len(source.Findings))
	for id, finding := range source.Findings {
		clone.Findings[id] = finding
	}
	return clone
}
