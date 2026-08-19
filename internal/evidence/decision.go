package evidence

type Finding struct {
	Severity       string `json:"severity"`
	File           string `json:"file"`
	Line           int    `json:"line"`
	Evidence       string `json:"evidence"`
	Recommendation string `json:"recommendation"`
}

type ReviewResult struct {
	EvidenceDigest string    `json:"evidenceDigest"`
	Findings       []Finding `json:"findings"`
}

type DecisionPacket struct {
	TaskDigest         string `json:"taskDigest"`
	EvidenceDigest     string `json:"evidenceDigest"`
	ReviewDigest       string `json:"reviewDigest,omitempty"`
	VerificationDigest string `json:"verificationDigest"`
}
