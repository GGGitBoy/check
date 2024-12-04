package apis

type Notify struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AppID      string `json:"app_id"`
	AppSecret  string `json:"app_secret"`
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
	Mobiles    string `json:"mobiles"`
	Emails     string `json:"emails"`
}

func NewNotify() *Notify {
	return &Notify{}
}

func NewNotifys() []*Notify {
	return []*Notify{}
}
