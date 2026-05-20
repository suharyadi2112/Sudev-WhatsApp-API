package model

import (
	"database/sql"
	"gowa-yourself/database"
	"strconv"
	"time"
)

// SimAttendance represents a daily attendance record
type SimAttendance struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"userId"`
	AttendanceDate time.Time `json:"attendanceDate"`
	PhoneNumber    string    `json:"phoneNumber,omitempty"`
	SimLabel       string    `json:"simLabel,omitempty"`
	AttendanceType string    `json:"attendanceType"`
	Status         string    `json:"status,omitempty"`
	Title          string    `json:"title"`
	Notes          string    `json:"notes,omitempty"`
	Metadata       string    `json:"metadata,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	InstanceID     string    `json:"instanceId,omitempty"`
}

// CreateAttendanceRequest for creating new attendance
type CreateAttendanceRequest struct {
	AttendanceDate string `json:"attendanceDate"` // YYYY-MM-DD or YYYY-MM-DD HH:MM:SS
	PhoneNumber    string `json:"phoneNumber"`
	SimLabel       string `json:"simLabel"`
	AttendanceType string `json:"attendanceType"`
	Status         string `json:"status"`
	Title          string `json:"title"`
	Notes          string `json:"notes"`
	Metadata       string `json:"metadata"`
}

// UpdateAttendanceRequest for updating attendance
type UpdateAttendanceRequest struct {
	PhoneNumber    *string `json:"phoneNumber"`
	SimLabel       *string `json:"simLabel"`
	AttendanceType *string `json:"attendanceType"`
	Status         *string `json:"status"`
	Title          *string `json:"title"`
	Notes          *string `json:"notes"`
	Metadata       *string `json:"metadata"`
}

// CreateAttendance creates a new attendance record
func CreateAttendance(userID int64, req *CreateAttendanceRequest) (*SimAttendance, error) {
	query := `
		INSERT INTO sim_attendance (
			user_id, attendance_date, phone_number, sim_label, 
			attendance_type, status, title, notes, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, user_id, attendance_date, phone_number, sim_label, 
		          attendance_type, status, title, notes, metadata, created_at, updated_at
	`

	var attendance SimAttendance
	var phoneNumber, simLabel, status, notes, metadata sql.NullString

	// Handle attendance date - default to current time if empty
	// If only date is provided (YYYY-MM-DD), append current time
	var attendanceDate interface{}
	if req.AttendanceDate == "" {
		attendanceDate = time.Now()
	} else if len(req.AttendanceDate) == 10 {
		currentTime := time.Now().Format(" 15:04:05")
		attendanceDate = req.AttendanceDate + currentTime
	} else {
		attendanceDate = req.AttendanceDate
	}

	// Handle empty metadata for JSONB compatibility
	metadataVal := req.Metadata
	if metadataVal == "" {
		metadataVal = "{}"
	}

	err := database.AppDB.QueryRow(
		query,
		userID,
		attendanceDate,
		req.PhoneNumber,
		req.SimLabel,
		req.AttendanceType,
		req.Status,
		req.Title,
		req.Notes,
		metadataVal,
	).Scan(
		&attendance.ID,
		&attendance.UserID,
		&attendance.AttendanceDate,
		&phoneNumber,
		&simLabel,
		&attendance.AttendanceType,
		&status,
		&attendance.Title,
		&notes,
		&metadata,
		&attendance.CreatedAt,
		&attendance.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if phoneNumber.Valid {
		attendance.PhoneNumber = phoneNumber.String
		// Fetch matching instance_id for convenience in response
		_ = database.AppDB.QueryRow("SELECT instance_id FROM instances WHERE phone_number = $1 LIMIT 1", phoneNumber.String).Scan(&attendance.InstanceID)
	}
	if simLabel.Valid {
		attendance.SimLabel = simLabel.String
	}
	if status.Valid {
		attendance.Status = status.String
	}
	if notes.Valid {
		attendance.Notes = notes.String
	}
	if metadata.Valid {
		attendance.Metadata = metadata.String
	}

	return &attendance, nil
}

// GetAttendances retrieves attendance records with optional filters including instanceID
func GetAttendances(userID int64, isAdmin bool, startDate, endDate, attendanceType, phoneNumber, simLabel, instanceID string) ([]SimAttendance, error) {
	var args []interface{}
	argCount := 0

	query := `
		SELECT s.id, s.user_id, s.attendance_date, s.phone_number, s.sim_label, 
		       s.attendance_type, s.status, s.title, s.notes, s.metadata, s.created_at, s.updated_at,
		       COALESCE(i.instance_id, '') AS instance_id
		FROM sim_attendance s
		LEFT JOIN instances i ON s.phone_number = i.phone_number
	`

	if !isAdmin {
		argCount++
		query += ` WHERE s.user_id = $` + strconv.Itoa(argCount)
		args = append(args, userID)
	} else {
		query += ` WHERE 1=1`
	}

	if startDate != "" {
		argCount++
		query += ` AND s.attendance_date >= $` + strconv.Itoa(argCount)
		if len(startDate) == 10 {
			args = append(args, startDate+" 00:00:00")
		} else {
			args = append(args, startDate)
		}
	}

	if endDate != "" {
		argCount++
		query += ` AND s.attendance_date <= $` + strconv.Itoa(argCount)
		if len(endDate) == 10 {
			args = append(args, endDate+" 23:59:59")
		} else {
			args = append(args, endDate)
		}
	}

	if attendanceType != "" {
		argCount++
		query += ` AND s.attendance_type = $` + strconv.Itoa(argCount)
		args = append(args, attendanceType)
	}

	if phoneNumber != "" {
		argCount++
		query += ` AND s.phone_number = $` + strconv.Itoa(argCount)
		args = append(args, phoneNumber)
	}

	if simLabel != "" {
		argCount++
		query += ` AND s.sim_label = $` + strconv.Itoa(argCount)
		args = append(args, simLabel)
	}

	if instanceID != "" {
		argCount++
		query += ` AND i.instance_id = $` + strconv.Itoa(argCount)
		args = append(args, instanceID)
	}

	query += ` ORDER BY s.attendance_date DESC, s.created_at DESC`

	rows, err := database.AppDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendances []SimAttendance

	for rows.Next() {
		var att SimAttendance
		var phoneNumber, simLabel, status, notes, metadata sql.NullString
		var dbInstanceID sql.NullString

		err := rows.Scan(
			&att.ID,
			&att.UserID,
			&att.AttendanceDate,
			&phoneNumber,
			&simLabel,
			&att.AttendanceType,
			&status,
			&att.Title,
			&notes,
			&metadata,
			&att.CreatedAt,
			&att.UpdatedAt,
			&dbInstanceID,
		)

		if err != nil {
			return nil, err
		}

		if phoneNumber.Valid {
			att.PhoneNumber = phoneNumber.String
		}
		if simLabel.Valid {
			att.SimLabel = simLabel.String
		}
		if status.Valid {
			att.Status = status.String
		}
		if notes.Valid {
			att.Notes = notes.String
		}
		if metadata.Valid {
			att.Metadata = metadata.String
		}
		if dbInstanceID.Valid {
			att.InstanceID = dbInstanceID.String
		}

		attendances = append(attendances, att)
	}

	return attendances, nil
}

// UpdateAttendance updates an existing attendance record
func UpdateAttendance(id int64, userID int64, req *UpdateAttendanceRequest) error {
	query := `
		UPDATE sim_attendance
		SET phone_number = COALESCE($1, phone_number),
		    sim_label = COALESCE($2, sim_label),
		    attendance_type = COALESCE($3, attendance_type),
		    status = COALESCE($4, status),
		    title = COALESCE($5, title),
		    notes = COALESCE($6, notes),
		    metadata = COALESCE($7, metadata),
		    updated_at = NOW()
		WHERE id = $8 AND user_id = $9
	`

	// Handle empty metadata for JSONB compatibility
	metadataVal := req.Metadata
	if metadataVal != nil && *metadataVal == "" {
		emptyJSON := "{}"
		metadataVal = &emptyJSON
	}

	result, err := database.AppDB.Exec(
		query,
		req.PhoneNumber,
		req.SimLabel,
		req.AttendanceType,
		req.Status,
		req.Title,
		req.Notes,
		metadataVal,
		id,
		userID,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteAttendance deletes an attendance record
func DeleteAttendance(id int64, userID int64) error {
	query := `DELETE FROM sim_attendance WHERE id = $1 AND user_id = $2`

	result, err := database.AppDB.Exec(query, id, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
