package telegram

type Post struct {
	SourceChannel string
	MessageID     int
	Text          string
	HasMedia      bool
	MediaKind     string
	FileID        string
	Caption       string
	GroupedID     int64
}
