package model

type MailAccount struct {
	Id      int    `storm:"id,increment" json:"i"  gorm:"primaryKey,autoIncrement"`
	Email   string `json:"e"`
	Account string `json:"a"`
	Passwd  string `json:"pw"`
	Host    string `json:"h"`
	Port    int    `json:"p"`
	Type    string `json:"t"`

	Status int    `json:"s"` //1: run 2:stop
	Info   string `json:"in"`

	RefreshToken string `json:"rt"`
	AccessToken  string `json:"at"`
	AccessExpire int64  `json:"ae"`

	InboxUidNext uint32
	SpamUidNext  uint32
}
type MailBox struct {
	Attributes []string `json:"a" gorm:"serializer:json"`
	Delimiter  string   `json:"d"`
	Name       string   `json:"n"`
}

// key email|box|uid
type Mail struct {
	Id             int      `storm:"id,increment" json:"i"  gorm:"primaryKey,autoIncrement"`
	Token          string   `storm:"index" json:"tk"`
	Email          string   `storm:"index" json:"e"`
	Mailbox        string   `storm:"index" json:"b"`
	Uid            uint32   `storm:"index" json:"u"`
	InternalDate   int64    `storm:"index" json:"idt"`
	Date           int64    `json:"dt"`
	Subject        string   `json:"s"`
	FromName       []string `json:"fn" gorm:"serializer:json"`
	FromAddress    []string `json:"fa" gorm:"serializer:json"`
	SenderName     []string `json:"sn" gorm:"serializer:json"`
	SenderAddress  []string `json:"sa" gorm:"serializer:json"`
	ToName         []string `json:"tn" gorm:"serializer:json"`
	ToAddress      []string `json:"ta" gorm:"serializer:json"`
	ReplyToName    []string `json:"rn" gorm:"serializer:json"`
	ReplyToAddress []string `json:"ra" gorm:"serializer:json"`
	CcName         []string `json:"cn" gorm:"serializer:json"`
	CcAddress      []string `json:"ca" gorm:"serializer:json"`
	BccName        []string `json:"bn" gorm:"serializer:json"`
	BccAddress     []string `json:"ba" gorm:"serializer:json"`
	InReplyTo      string   `json:"ir"`
	MessageID      string   `json:"m"`

	Text string `json:"t"`
	Html string `json:"h"`

	Attatchment map[string]Attatchment `json:"a" gorm:"serializer:json"`
}

type Attatchment struct {
	Path  string `json:"p"`
	Mime  string `json:"m"`
	Name  string `json:"n"`
	Value []byte `json:"v"`
}

type Address struct {
	Name    string `json:"n"`
	Mailbox string `json:"m"`
	Host    string `json:"h"`
}
type TmpUid struct {
	Id   int    `storm:"id,increment"  gorm:"primaryKey,autoIncrement"`
	Date int64  `storm:"index" json:"dt"`
	Uid  uint32 `json:"u"`
}
