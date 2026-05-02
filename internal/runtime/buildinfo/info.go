package buildinfo

type Info struct {
	RuntimeKind string
	Version     string
	Commit      string
	Host        string
	Port        int
	DataDir     string
}
