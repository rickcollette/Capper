package s3server

import "encoding/xml"

// S3 error codes
const (
	ErrCodeNoSuchBucket           = "NoSuchBucket"
	ErrCodeNoSuchKey              = "NoSuchKey"
	ErrCodeBucketNotEmpty         = "BucketNotEmpty"
	ErrCodeBucketAlreadyExists    = "BucketAlreadyExists"
	ErrCodeInvalidBucketName      = "InvalidBucketName"
	ErrCodeAccessDenied           = "AccessDenied"
	ErrCodeInternalError          = "InternalError"
	ErrCodeBucketQuotaExceeded    = "BucketQuotaExceeded"
)

// Sentinel errors returned by ObjectService.
var (
	ErrNoSuchKey           = &S3Err{Code: ErrCodeNoSuchKey, Message: "The specified key does not exist."}
	ErrNoSuchBucket        = &S3Err{Code: ErrCodeNoSuchBucket, Message: "The specified bucket does not exist."}
	ErrBucketNotEmpty      = &S3Err{Code: ErrCodeBucketNotEmpty, Message: "The bucket you tried to delete is not empty."}
	ErrAccessDenied        = &S3Err{Code: ErrCodeAccessDenied, Message: "Access Denied."}
	ErrBucketQuotaExceeded = &S3Err{Code: ErrCodeBucketQuotaExceeded, Message: "Bucket quota exceeded."}
)

// S3Err is a typed error carrying an S3 error code.
type S3Err struct {
	Code    string
	Message string
}

func (e *S3Err) Error() string { return e.Code + ": " + e.Message }

// S3Error is the XML error response body per the S3 spec.
type S3Error struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestID string   `xml:"RequestId,omitempty"`
}

// ListAllMyBucketsResult is the response body for GET / (list buckets).
type ListAllMyBucketsResult struct {
	XMLName xml.Name  `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListAllMyBucketsResult"`
	Owner   S3Owner   `xml:"Owner"`
	Buckets S3Buckets `xml:"Buckets"`
}

// S3Owner is the owner element.
type S3Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

// S3Buckets wraps the bucket list.
type S3Buckets struct {
	Bucket []S3Bucket `xml:"Bucket"`
}

// S3Bucket is a single entry in the bucket list.
type S3Bucket struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
}

// ListBucketResult is the response body for GET /<bucket> (list objects).
type ListBucketResult struct {
	XMLName        xml.Name          `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name           string            `xml:"Name"`
	Prefix         string            `xml:"Prefix"`
	Delimiter      string            `xml:"Delimiter,omitempty"`
	MaxKeys        int               `xml:"MaxKeys"`
	IsTruncated    bool              `xml:"IsTruncated"`
	KeyCount       int               `xml:"KeyCount"`
	Contents       []S3ObjectContent `xml:"Contents"`
	CommonPrefixes []S3CommonPrefix  `xml:"CommonPrefixes,omitempty"`
}

// S3ObjectContent is a single object entry in a listing.
type S3ObjectContent struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

// S3CommonPrefix is a virtual folder in a listing.
type S3CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}
