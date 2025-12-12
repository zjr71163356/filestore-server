package dao

import (
	"context"
	"database/sql"
	"filestore-server/pkg/db"
	"fmt"
)

type FileMeta struct {
	FileSha1 string
	FileName string
	FileSize int64
	Location string
	UploadAt string
}

func SaveFileMeta(ctx context.Context, filehash string, filename string, filesize int64, fileaddr string) error {
	const sqlStr = "insert ignore into tbl_file (`file_sha1`,`file_name`,`file_size`,`file_addr`,`status`) values(?,?,?,?,1)"

	conn := db.DBconn()
	if conn == nil {
		return fmt.Errorf("db connection is nil")
	}

	result, err := conn.ExecContext(ctx, sqlStr, filehash, filename, filesize, fileaddr)
	if err != nil {
		return fmt.Errorf("failed to insert file meta: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows <= 0 {
		return fmt.Errorf("file with hash %s uploaded before", filehash)
	}

	return nil

}

func GetFileMeta(ctx context.Context, filehash string) (FileMeta, error) {
	const sqlStr = "select file_sha1,file_addr,file_name,file_size from tbl_file where file_sha1=? and status=1 limit 1"

	conn := db.DBconn()
	if conn == nil {
		return FileMeta{}, fmt.Errorf("db connection is nil")
	}

	tableFile := FileMeta{}

	err := conn.QueryRowContext(ctx, sqlStr, filehash).Scan(&tableFile.FileSha1, &tableFile.Location, &tableFile.FileName, &tableFile.FileSize)
	if err != nil {
		if err == sql.ErrNoRows {
			// 查不到记录，返回空结构体和 nil 错误
			return FileMeta{}, fmt.Errorf("file not found")
		}
		return FileMeta{}, fmt.Errorf("failed to query file meta: %w", err)
	}

	return tableFile, nil
}

// UpdateFileMeta 更新文件的元信息（目前支持文件名、存储路径、大小）
func UpdateFileMeta(ctx context.Context, fmeta FileMeta) error {
	const sqlStr = "update tbl_file set file_name=?, file_size=?, file_addr=? where file_sha1=? and status=1"

	conn := db.DBconn()
	if conn == nil {
		return fmt.Errorf("db connection is nil")
	}

	result, err := conn.ExecContext(ctx, sqlStr, fmeta.FileName, fmeta.FileSize, fmeta.Location, fmeta.FileSha1)
	if err != nil {
		return fmt.Errorf("failed to update file meta: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("file %s not found or not active", fmeta.FileSha1)
	}

	return nil
}
