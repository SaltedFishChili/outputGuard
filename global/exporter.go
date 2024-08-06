package global

type ExporterData struct {
	ChainName string
	Ip        string
	Packets   float64
	Bytes     float64
	Hostname  string
}

var (
	ExporterDatas = make(chan ExporterData, 100000)
	RunSig        = make(chan struct{})
)
