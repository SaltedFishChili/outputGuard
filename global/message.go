package global

type Messages struct {
	IP         string `json:"ip"`
	Action     string `json:"action"`
	IsLocalNet bool
}
