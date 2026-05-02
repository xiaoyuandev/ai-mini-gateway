package buildinfo

const (
	DefaultRuntimeKind     = "ai-mini-gateway"
	DefaultVersion         = "dev"
	DefaultCommit          = "unknown"
	DefaultContractVersion = "v1"
)

type Info struct {
	RuntimeKind     string
	Version         string
	Commit          string
	ContractVersion string
	Host            string
	Port            int
	DataDir         string
}
