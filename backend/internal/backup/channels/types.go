package channels

import "context"

type ID string

const OpenList ID = "openlist"
const S3 ID = "s3"

const (
	MaxBackupMetadataBytes              = 64 * 1024 * 1024
	UploadProgressStageAwaitingResponse = "awaiting-response"
	UploadProgressStageVerifying        = "verifying"
)

type File struct {
	Name       string
	Size       int64
	ModifiedAt string
	Directory  bool
}

type UploadOutcome struct {
	File    File
	Warning string
}

type UploadProgress struct {
	BytesTransferred int64
	TotalBytes       int64
	BytesPerSecond   float64
	Stage            string
}

type UploadProgressFunc func(UploadProgress)

type Client interface {
	ID() ID
	Test(context.Context) error
	List(context.Context) ([]File, error)
	Upload(context.Context, string, string) (File, error)
	UploadMetadata(context.Context, string, string) (File, error)
	Download(context.Context, string, string) error
	DownloadMetadata(context.Context, string, string) error
}

type ProgressClient interface {
	Client
	UploadWithProgress(context.Context, string, string, UploadProgressFunc) (File, error)
	UploadMetadataWithProgress(context.Context, string, string, UploadProgressFunc) (File, error)
}

type UploadOutcomeClient interface {
	UploadWithProgressOutcome(context.Context, string, string, UploadProgressFunc) (UploadOutcome, error)
}
