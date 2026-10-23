package skills

// NewProber is a handshake cache outside a deploy run, the same one a deploy run owns: one probe per executable for the prober's lifetime.
func NewProber() Prober {
	return &deployRun{}
}
