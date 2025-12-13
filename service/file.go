package service

import (
	"context"
	"filestore-server/pkg/dao"
	"fmt"
)

// 直接使用 DAO（数据库）持久化。
func SaveFileMeta(ctx context.Context, fmeta dao.FileMeta) error {
	if fmeta.FileSha1 == "" {
		return fmt.Errorf("file sha1 is required")
	}
	return dao.SaveFileMeta(ctx, fmeta.FileSha1, fmeta.FileName, fmeta.FileSize, fmeta.Location)
}

func GetFileMeta(ctx context.Context, filehash string) (dao.FileMeta, error) {
	if filehash == "" {
		return dao.FileMeta{}, fmt.Errorf("filehash is required")
	}
	return dao.GetFileMeta(ctx, filehash)
}

func UpdateFileMeta(ctx context.Context, fmeta dao.FileMeta) error {
	if fmeta.FileSha1 == "" {
		return fmt.Errorf("file sha1 is required")
	}
	return dao.UpdateFileMeta(ctx, fmeta)
}

func DeleteFileMeta(ctx context.Context, filehash string) error {
	if filehash == "" {
		return fmt.Errorf("filehash is required")
	}
	return dao.DeleteFileMeta(ctx, filehash)
}
