package meta

type FileMeta struct {
	FileSha1 string
	FileName string
	FileSize int64
	Location string
	UploadAt string
}

var fileMetaMap map[string]FileMeta

func init() {
	fileMetaMap = make(map[string]FileMeta)
}

func UpdateFileMeta(fmeta FileMeta) {
	fileMetaMap[fmeta.FileSha1] = fmeta
}

func GetFileMeta(filesha1 string) FileMeta {
	return fileMetaMap[filesha1]
}
