package parser

type Payload struct {
	Value string `json:"value"`
}

type Action struct {
	Action  string  `json:"action"`
	Payload Payload `json:"payload"`
}
