package service

type ResCode int64

const (
	CodeSuccess ResCode = 1000
)

var msgFlags = map[ResCode]string{
	CodeSuccess: "success",
}

func (c ResCode) Msg() string {
	msg, ok := msgFlags[c]
	if ok {
		return msg
	}
	return ""
}
