package buildinfo

type Info struct {
	RuntimeKind     string
	Version         string
	Commit          string
	ContractVersion string
	Host            string
	Port            int
	DataDir         string
}
