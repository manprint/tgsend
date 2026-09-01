package message

// Entity identifies a UTF-16 range in a message chunk.
type Entity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

// Chunk is one Telegram message request before transport-specific encoding.
type Chunk struct {
	Text                string
	Entities            []Entity
	DisableNotification bool
}
