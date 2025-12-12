package meta

import (
	"context"
	"filestore-server/pkg/dao"
	"log"
)

var fileMetaMap map[string]dao.FileMeta

func init() {
	fileMetaMap = make(map[string]dao.FileMeta)
}

func InsertFileMeta(fmeta dao.FileMeta) {
	// fileMetaMap[fmeta.FileSha1] = fmeta

	if err := dao.SaveFileMeta(context.Background(), fmeta.FileSha1, fmeta.FileName, fmeta.FileSize, fmeta.Location); err != nil {
		log.Printf("failed to persist file meta to db: %v", err)
	}
}

func UpdateFileMeta(fmeta dao.FileMeta) {

}

func GetFileMeta(filesha1 string) (dao.FileMeta, bool) {
	if fmeta, exists := fileMetaMap[filesha1]; exists {
		return fmeta, true
	}

	tableFile, err := dao.GetFileMeta(context.Background(), filesha1)
	if err != nil {
		log.Printf("failed to load file meta from db: %v", err)
		return dao.FileMeta{}, false
	}

	// fileMetaMap[filesha1] = fmeta

	return tableFile, true
}

func RemoveFileMeta(filesha1 string) {

	delete(fileMetaMap, filesha1)
}
