package browser

import "context"

type DownloadTaskState struct {
	TaskID              string  `json:"taskId"`
	Phase               string  `json:"phase"`
	Version             string  `json:"version"`
	AssetName           string  `json:"assetName"`
	DownloadedBytes     int64   `json:"downloadedBytes"`
	TotalBytes          int64   `json:"totalBytes"`
	Progress            float64 `json:"progress"`
	SpeedBytesPerSecond int64   `json:"speedBytesPerSecond"`
	EstimatedSeconds    int64   `json:"estimatedSeconds"`
	Message             string  `json:"message"`
	ErrorCode           string  `json:"errorCode"`
	ErrorDetail         string  `json:"errorDetail"`
	CanRetry            bool    `json:"canRetry"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
}

// DownloadProgress 进度信息载体
type DownloadProgress struct {
	TaskID   string `json:"taskId,omitempty"`
	Phase    string `json:"phase"`    // "downloading" 或 "extracting" 或 "done" 或 "error"
	Progress int    `json:"progress"` // 进度百分比 0-100
	Message  string `json:"message"`  // 附加详情
}

type coreDownloadWriter struct {
	writeFunc func(p []byte) (n int, err error)
	ctx       context.Context
}

func (cw *coreDownloadWriter) Write(p []byte) (int, error) {
	select {
	case <-cw.ctx.Done():
		return 0, cw.ctx.Err()
	default:
	}
	return cw.writeFunc(p)
}
