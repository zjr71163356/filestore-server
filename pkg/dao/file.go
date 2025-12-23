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

func SaveFileMeta(ctx context.Context, fileHash string, filename string, filesize int64, fileaddr string) error {
	const sqlStr = "insert ignore into tbl_file (`file_sha1`,`file_name`,`file_size`,`file_addr`,`status`) values(?,?,?,?,0)"

	conn := db.DBconn()
	if conn == nil {
		return fmt.Errorf("db connection is nil")
	}

	result, err := conn.ExecContext(ctx, sqlStr, fileHash, filename, filesize, fileaddr)
	if err != nil {
		return fmt.Errorf("failed to insert file meta: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows <= 0 {
		return fmt.Errorf("file with hash %s uploaded before", fileHash)
	}

	return nil

}

func GetFileMeta(ctx context.Context, fileHash string) (FileMeta, error) {
	const sqlStr = "select file_sha1,file_addr,file_name,file_size from tbl_file where file_sha1=? and status=0 limit 1"

	conn := db.DBconn()
	if conn == nil {
		return FileMeta{}, fmt.Errorf("db connection is nil")
	}

	tableFile := FileMeta{}

	err := conn.QueryRowContext(ctx, sqlStr, fileHash).Scan(&tableFile.FileSha1, &tableFile.Location, &tableFile.FileName, &tableFile.FileSize)
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
	const sqlStr = "update tbl_file set file_name=?, file_size=?, file_addr=? where file_sha1=? and status=0"

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

// DeleteFileMeta 软删除（将 status 置为 1）
func DeleteFileMeta(ctx context.Context, fileHash string) error {
	const sqlStr = "update tbl_file set status=1 where file_sha1=?"

	conn := db.DBconn()
	if conn == nil {
		return fmt.Errorf("db connection is nil")
	}

	result, err := conn.ExecContext(ctx, sqlStr, fileHash)
	if err != nil {
		return fmt.Errorf("failed to delete file meta: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("file %s not found", fileHash)
	}
	return nil
}

// RestoreFileMeta 将 status 从 1 恢复为 0，用于删除用例的失败补偿。
func RestoreFileMeta(ctx context.Context, fileHash string) error {
	const sqlStr = "update tbl_file set status=0 where file_sha1=? and status=1"

	conn := db.DBconn()
	if conn == nil {
		return fmt.Errorf("db connection is nil")
	}

	result, err := conn.ExecContext(ctx, sqlStr, fileHash)
	if err != nil {
		return fmt.Errorf("failed to restore file meta: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("file %s not found or not deleted", fileHash)
	}

	return nil
}

func InsertUserFileMeta(ctx context.Context, username, fileSha1 string, fileSize int64, fileName string) error {
	const sqlStr = "insert ignore into tbl_user_file (`user_name`,`file_sha1`,`file_size`,`file_name`,`status`) values (?,?,?,?,0)"
	conn := db.DBconn()
	if conn == nil {
		return fmt.Errorf("db connection is nil")
	}

	result, err := conn.ExecContext(ctx, sqlStr, username, fileSha1, fileSize, fileName)

	if err != nil {
		return fmt.Errorf("failed to update user file meta: %w", err)
	}

	rows, err := result.RowsAffected()

	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows <= 0 {
		return nil
	}

	return nil
}

func GetUserFilelist(ctx context.Context, username string, limit, offset int) ([]FileMeta, int, error) {
	conn := db.DBconn()
	if conn == nil {
		return nil, 0, fmt.Errorf("db connection is nil")
	}

	const countSQL = "select count(*) from tbl_user_file where user_name=? and status=0"
	var total int
	if err := conn.QueryRowContext(ctx, countSQL, username).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count user files: %w", err)
	}

	const sqlStr = "select file_sha1,file_name,file_size,last_update from tbl_user_file where user_name=? and status=0 order by last_update desc, id desc limit ? offset ?"
	rows, err := conn.QueryContext(ctx, sqlStr, username, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query user file list: %w", err)
	}
	defer rows.Close()

	var fileMetaList []FileMeta
	for rows.Next() {

		var f FileMeta
		var lastUpdate sql.NullTime
		err := rows.Scan(&f.FileSha1, &f.FileName, &f.FileSize, &lastUpdate)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan row: %w", err)
		}
		if lastUpdate.Valid {
			f.UploadAt = lastUpdate.Time.Format("2006-01-02 15:04:05")
		}
		fileMetaList = append(fileMetaList, f)

	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return fileMetaList, total, nil
}

func GetFileExist(ctx context.Context, filehash string) (FileMeta, bool, error) {
	conn := db.DBconn()
	if conn == nil {
		return FileMeta{}, false, fmt.Errorf("db connection is nil")
	}

	// SQL 关键字不区分大小写，但表名 tbl_file 在 Linux 下通常区分
	const sqlStr = "select file_sha1,file_name,file_size,file_addr from tbl_file where file_sha1=? and status=0 limit 1"

	var fmeta FileMeta
	err := conn.QueryRowContext(ctx, sqlStr, filehash).Scan(&fmeta.FileSha1, &fmeta.FileName, &fmeta.FileSize, &fmeta.Location)
	if err != nil {
		if err == sql.ErrNoRows {
			return FileMeta{}, false, nil
		}
		return FileMeta{}, false, fmt.Errorf("failed to query file meta: %w", err)
	}

	return fmeta, true, nil
}
