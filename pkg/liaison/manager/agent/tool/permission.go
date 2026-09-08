package tool

// ResourcePermission declares the operation-level IAM check. Executors must
// additionally enforce visibility of every concrete resource and reference.
type ResourcePermission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}
