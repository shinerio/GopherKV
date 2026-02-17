package protocol

const (
	CodeSuccess       = 0
	CodeKeyNotFound   = 1001
	CodeKeyExpired    = 1002
	CodeKeyTooLong    = 2001
	CodeValueTooLarge = 2002
	CodeInvalidParam  = 2003
	CodeMemoryFull    = 3001
	CodeInternalError = 5001
)

var CodeMessages = map[int]string{
	CodeSuccess:       "ok",
	CodeKeyNotFound:   "key not found",
	CodeKeyExpired:    "key expired",
	CodeKeyTooLong:    "key too long",
	CodeValueTooLarge: "value too large",
	CodeInvalidParam:  "invalid parameter",
	CodeMemoryFull:    "memory full",
	CodeInternalError: "internal error",
}

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

type SetRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   int    `json:"ttl,omitempty"`
}

type GetResponseData struct {
	Value        string `json:"value"`
	TTLRemaining int    `json:"ttl_remaining,omitempty"`
}

type TTLResponseData struct {
	TTL int `json:"ttl"`
}

type StatsResponseData struct {
	Keys     int              `json:"keys"`
	Memory   int64            `json:"memory"`
	Hits     int64            `json:"hits"`
	Misses   int64            `json:"misses"`
	Requests map[string]int64 `json:"requests"`
	Uptime   int64            `json:"uptime"`
}

type SnapshotResponseData struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

type HealthResponseData struct {
	Status string `json:"status"`
}
