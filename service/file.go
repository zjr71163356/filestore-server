package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"filestore-server/pkg/dao"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

// UploadFile 编排上传用例：落盘 + 写入元信息；DB 失败会回滚文件。
func UploadFile(ctx context.Context, src io.Reader, filename string) (dao.FileMeta, error) {
	if err := os.MkdirAll("./tmp", 0o755); err != nil {
		return dao.FileMeta{}, fmt.Errorf("failed to create tmp dir: %w", err)
	}

	location := "./tmp/" + filename
	dst, err := os.Create(location)
	if err != nil {
		return dao.FileMeta{}, fmt.Errorf("failed to create file: %w", err)
	}

	shouldCleanup := true
	defer func() {
		_ = dst.Close()
		if shouldCleanup {
			_ = os.Remove(location)
		}
	}()

	hash := sha1.New()
	filesize, err := io.Copy(io.MultiWriter(dst, hash), src)
	if err != nil {
		return dao.FileMeta{}, fmt.Errorf("failed to save file: %w", err)
	}
	fileSha1 := hex.EncodeToString(hash.Sum(nil))

	fmeta := dao.FileMeta{
		FileSha1: fileSha1,
		FileName: filename,
		FileSize: filesize,
		Location: location,
		UploadAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	if err := dao.SaveFileMeta(ctx, fmeta.FileSha1, fmeta.FileName, fmeta.FileSize, fmeta.Location); err != nil {
		return dao.FileMeta{}, err
	}

	shouldCleanup = false
	return fmeta, nil
}

// DownloadFile 编排下载用例：查询元信息 + 读取文件内容。
func DownloadFile(ctx context.Context, filehash string) (dao.FileMeta, []byte, error) {
	fmeta, err := dao.GetFileMeta(ctx, filehash)
	if err != nil {
		return dao.FileMeta{}, nil, err
	}

	data, err := os.ReadFile(fmeta.Location)
	if err != nil {
		return dao.FileMeta{}, nil, fmt.Errorf("failed to read file: %w", err)
	}

	return fmeta, data, nil
}

// RenameFile 编排重命名用例：读取元信息 + 更新文件名。
func RenameFile(ctx context.Context, filehash, newFilename string) (dao.FileMeta, error) {
	fmeta, err := dao.GetFileMeta(ctx, filehash)
	if err != nil {
		return dao.FileMeta{}, err
	}
	fmeta.FileName = newFilename

	if err := dao.UpdateFileMeta(ctx, fmeta); err != nil {
		return dao.FileMeta{}, err
	}
	return fmeta, nil
}

// DeleteFile 编排删除用例：先移走文件再删除元信息，失败可回滚。
func DeleteFile(ctx context.Context, filehash string) error {
	fmeta, err := dao.GetFileMeta(ctx, filehash)
	if err != nil {
		return err
	}
	if fmeta.Location == "" {
		return fmt.Errorf("file location is empty")
	}

	trashPath := fmeta.Location + ".trash." + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.Rename(fmeta.Location, trashPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	if err := dao.DeleteFileMeta(ctx, filehash); err != nil {
		_ = os.Rename(trashPath, fmeta.Location)
		return err
	}

	if err := os.Remove(trashPath); err != nil {
		// 尝试补偿：恢复元信息并把文件放回去。
		_ = dao.RestoreFileMeta(ctx, filehash)
		_ = os.Rename(trashPath, fmeta.Location)
		return fmt.Errorf("failed to remove file: %w", err)
	}

	return nil
}

func UpdateUserFileMeta(ctx context.Context, username string, fmeta dao.FileMeta) {
	dao.UpdateUserFileMeta(ctx, username, fmeta)
}
