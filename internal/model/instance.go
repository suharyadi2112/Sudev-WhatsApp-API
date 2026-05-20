package model

import (
	"database/sql"
	"errors"
	"fmt"
	"gowa-yourself/database"
	"log"
	"time"
)

// Struct Instance sesuai field table
type Instance struct {
	ID              int64
	InstanceID      string
	PhoneNumber     sql.NullString
	JID             sql.NullString
	Status          string
	IsConnected     bool
	Name            sql.NullString
	ProfilePicture  sql.NullString
	About           sql.NullString
	Platform        sql.NullString
	BatteryLevel    sql.NullInt64
	BatteryCharging sql.NullBool
	QRCode          sql.NullString
	QRExpiresAt     sql.NullTime
	CreatedAt       time.Time
	ConnectedAt     sql.NullTime
	DisconnectedAt  sql.NullTime
	LastSeen        sql.NullTime
	SessionData     []byte
	Circle          string
	WebhookURL      sql.NullString
	WebhookSecret   sql.NullString
	Used            bool           `json:"used"`
	Keterangan      sql.NullString `json:"keterangan"`
	CreatedBy       sql.NullInt64  `json:"created_by"`
}

type InstanceResp struct {
	ID                int64     `json:"id"`
	InstanceID        string    `json:"instanceId"`
	PhoneNumber       string    `json:"phoneNumber"`
	JID               string    `json:"jid"`
	Status            string    `json:"status"`
	IsConnected       bool      `json:"isConnected"`
	Name              string    `json:"name"`
	ProfilePicture    string    `json:"profilePicture"`
	About             string    `json:"about"`
	Platform          string    `json:"platform"`
	BatteryLevel      int64     `json:"batteryLevel"`
	BatteryCharging   bool      `json:"batteryCharging"`
	QRCode            string    `json:"qrCode"`
	QRExpiresAt       time.Time `json:"qrExpiresAt"`
	CreatedAt         time.Time `json:"createdAt"`
	ConnectedAt       time.Time `json:"connectedAt"`
	DisconnectedAt    time.Time `json:"disconnectedAt"`
	LastSeen          time.Time `json:"lastSeen"`
	ExistsInWhatsmeow bool      `json:"existsInWhatsmeow"`
	Circle            string    `json:"circle"`
	Used              bool      `json:"used"`
	Keterangan        string    `json:"keterangan"`
	CreatedBy         int64     `json:"createdBy,omitempty"`
}

var ErrNoActiveInstance = errors.New("no active instance for this phone number")

// GetActiveInstanceByPhoneNumber mengembalikan instance aktif (terbaru) untuk nomor tertentu.
func GetActiveInstanceByPhoneNumber(phoneNumber string) (*Instance, error) {
	query := `
        SELECT
            id,
            instance_id,
            phone_number,
            jid,
            status,
            is_connected,
            name,
            profile_picture,
            about,
            platform,
            battery_level,
            battery_charging,
            qr_code,
            qr_expires_at,
            created_at,
            connected_at,
            disconnected_at,
            last_seen,
            session_data
        FROM instances
        WHERE phone_number = $1
          AND status = 'online'
          AND is_connected = true
        ORDER BY connected_at DESC, created_at DESC
        LIMIT 1
    `

	inst := &Instance{}
	err := database.AppDB.QueryRow(query, phoneNumber).Scan(
		&inst.ID,
		&inst.InstanceID,
		&inst.PhoneNumber,
		&inst.JID,
		&inst.Status,
		&inst.IsConnected,
		&inst.Name,
		&inst.ProfilePicture,
		&inst.About,
		&inst.Platform,
		&inst.BatteryLevel,
		&inst.BatteryCharging,
		&inst.QRCode,
		&inst.QRExpiresAt,
		&inst.CreatedAt,
		&inst.ConnectedAt,
		&inst.DisconnectedAt,
		&inst.LastSeen,
		&inst.SessionData,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoActiveInstance
		}
		return nil, err
	}

	return inst, nil
}

// insert informasi instance ke table database custom
func InsertInstance(in *Instance) error {
	query := `
    INSERT INTO instances (
        instance_id, status, is_connected, created_at, session_data, circle, used, created_by
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := database.AppDB.Exec(
		query,
		in.InstanceID,
		in.Status,
		in.IsConnected,
		in.CreatedAt,
		in.SessionData,
		in.Circle,
		true, // Default used = true
		in.CreatedBy,
	)
	return err
}

// update status qr seperti expired dll
func UpdateInstanceQR(instanceID, qr string, expiresAt time.Time) error {
	query := `
        UPDATE instances
        SET qr_code = $1, qr_expires_at = $2, status = $3
        WHERE instance_id = $4
    `
	_, err := database.AppDB.Exec(query, qr, expiresAt, "qr_required", instanceID)
	return err
}

// Ambil semua instance dari database custom
func GetAllInstances() ([]Instance, error) {
	query := `
        SELECT 
            id,
            instance_id,
            phone_number,
            jid,
            status,
            is_connected,
            name,
            profile_picture,
            about,
            platform,
            battery_level,
            battery_charging,
            qr_code,
            qr_expires_at,
            created_at,
            connected_at,
            disconnected_at,
            last_seen,
            session_data,
			circle,
			used,
			keterangan,
			created_by
        FROM instances
        ORDER BY 
            CASE WHEN circle = 'one' THEN 0 ELSE 1 END,
            circle ASC,
            used DESC, 
            is_connected DESC, 
            created_at DESC
    `

	rows, err := database.AppDB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []Instance
	for rows.Next() {
		var inst Instance

		err = rows.Scan(
			&inst.ID,
			&inst.InstanceID,
			&inst.PhoneNumber,
			&inst.JID,
			&inst.Status,
			&inst.IsConnected,
			&inst.Name,
			&inst.ProfilePicture,
			&inst.About,
			&inst.Platform,
			&inst.BatteryLevel,
			&inst.BatteryCharging,
			&inst.QRCode,
			&inst.QRExpiresAt,
			&inst.CreatedAt,
			&inst.ConnectedAt,
			&inst.DisconnectedAt,
			&inst.LastSeen,
			&inst.SessionData,
			&inst.Circle,
			&inst.Used,
			&inst.Keterangan,
			&inst.CreatedBy,
		)

		if err != nil {
			log.Println("Scan error GetAllInstances():", err)
			continue
		}

		instances = append(instances, inst)
	}

	return instances, nil
}

// update untuk eventHandler whatsapp
func UpdateInstanceOnConnected(instanceID, jid, phoneNumber, platform string) error {
	query := `
        UPDATE instances
        SET
            jid = $1,
            phone_number = $2,
            platform = $3,
            status = 'online',
            is_connected = true,
            connected_at = NOW(),
            last_seen = NOW()
        WHERE instance_id = $4
    `
	_, err := database.AppDB.Exec(query, jid, phoneNumber, platform, instanceID)
	return err
}

func UpdateInstanceOnDisconnected(instanceID string) error {
	query := `
        UPDATE instances
        SET
            status = 'disconnected',
            is_connected = false,
            disconnected_at = NOW()
        WHERE instance_id = $1
    `
	_, err := database.AppDB.Exec(query, instanceID)
	return err
}

func UpdateInstanceOnLoggedOut(instanceID string) error {
	query := `
        UPDATE instances
        SET
            status = 'logged_out',
            is_connected = false,
            disconnected_at = NOW()
        WHERE instance_id = $1
    `
	_, err := database.AppDB.Exec(query, instanceID)
	return err
}

// update status by logout api
func UpdateInstanceStatus(instanceID, status string, isConnected bool, disconnectedAt time.Time) error {
	query := `
        UPDATE instances
        SET status = $1, is_connected = $2, disconnected_at = $3
        WHERE instance_id = $4
    `
	_, err := database.AppDB.Exec(query, status, isConnected, disconnectedAt, instanceID)
	return err
}

// Get instance by JID
func GetInstanceByJID(jid string) (*Instance, error) {

	query := `
        SELECT
            id,
            instance_id,
            phone_number,
            jid,
            status,
            is_connected,
            name,
            profile_picture,
            about,
            platform,
            battery_level,
            battery_charging,
            qr_code,
            qr_expires_at,
            created_at,
            connected_at,
            disconnected_at,
            last_seen,
            session_data
        FROM instances
        WHERE jid = $1
        ORDER BY created_at DESC
        LIMIT 1
    `

	inst := &Instance{}
	err := database.AppDB.QueryRow(query, jid).Scan(
		&inst.ID,
		&inst.InstanceID,
		&inst.PhoneNumber,
		&inst.JID,
		&inst.Status,
		&inst.IsConnected,
		&inst.Name,
		&inst.ProfilePicture,
		&inst.About,
		&inst.Platform,
		&inst.BatteryLevel,
		&inst.BatteryCharging,
		&inst.QRCode,
		&inst.QRExpiresAt,
		&inst.CreatedAt,
		&inst.ConnectedAt,
		&inst.DisconnectedAt,
		&inst.LastSeen,
		&inst.SessionData,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err // biar caller bisa bedakan ErrNoRows
		}
		return nil, fmt.Errorf("GetInstanceByJID scan error: %w", err)
	}

	return inst, nil
}

// Get instance by INSTANCE ID
func GetInstanceByInstanceID(instanceID string) (*Instance, error) {

	query := `
        SELECT
            id,
            instance_id,
            phone_number,
            jid,
            status,
            is_connected,
            name,
            profile_picture,
            about,
            platform,
            battery_level,
            battery_charging,
            qr_code,
            qr_expires_at,
            created_at,
            connected_at,
            disconnected_at,
            last_seen,
            session_data,
			webhook_url,
			webhook_secret,
			used,
			keterangan,
			created_by
        FROM instances
        WHERE instance_id = $1
        LIMIT 1
    `

	inst := &Instance{}

	var (
		jidNS            sql.NullString
		phoneNS          sql.NullString
		nameNS           sql.NullString
		profileNS        sql.NullString
		aboutNS          sql.NullString
		platformNS       sql.NullString
		qrCodeNS         sql.NullString
		qrExpiresAtNT    sql.NullTime
		connectedAtNT    sql.NullTime
		disconnectedAtNT sql.NullTime
		lastSeenNT       sql.NullTime
	)

	err := database.AppDB.QueryRow(query, instanceID).Scan(
		&inst.ID,
		&inst.InstanceID,
		&phoneNS,
		&jidNS,
		&inst.Status,
		&inst.IsConnected,
		&nameNS,
		&profileNS,
		&aboutNS,
		&platformNS,
		&inst.BatteryLevel,
		&inst.BatteryCharging,
		&qrCodeNS,
		&qrExpiresAtNT,
		&inst.CreatedAt,
		&connectedAtNT,
		&disconnectedAtNT,
		&lastSeenNT,
		&inst.SessionData,
		&inst.WebhookURL,
		&inst.WebhookSecret,
		&inst.Used,
		&inst.Keterangan,
		&inst.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	// Assign dari variabel Null* ke field struct
	inst.QRCode = qrCodeNS           // ← tambahkan baris ini
	inst.QRExpiresAt = qrExpiresAtNT // ← dan ini

	return inst, nil
}

// Hapus instance table custom
func DeleteInstanceByInstanceID(instanceID string) error {
	_, err := database.AppDB.Exec(`DELETE FROM instances WHERE instance_id = $1`, instanceID)
	return err
}

func ToResponse(inst Instance) InstanceResp {
	resp := InstanceResp{
		ID:              inst.ID,
		InstanceID:      inst.InstanceID,
		JID:             inst.JID.String,
		Status:          inst.Status,
		IsConnected:     inst.IsConnected,
		BatteryLevel:    0,
		BatteryCharging: false,
		Circle:          inst.Circle,
	}

	if inst.PhoneNumber.Valid {
		resp.PhoneNumber = inst.PhoneNumber.String
	}
	if inst.Name.Valid {
		resp.Name = inst.Name.String
	}
	if inst.ProfilePicture.Valid {
		resp.ProfilePicture = inst.ProfilePicture.String
	}
	if inst.About.Valid {
		resp.About = inst.About.String
	}
	if inst.Platform.Valid {
		resp.Platform = inst.Platform.String
	}
	if inst.BatteryLevel.Valid {
		resp.BatteryLevel = inst.BatteryLevel.Int64
	}
	if inst.BatteryCharging.Valid {
		resp.BatteryCharging = inst.BatteryCharging.Bool
	}
	if inst.QRCode.Valid {
		resp.QRCode = inst.QRCode.String
	}
	if inst.QRExpiresAt.Valid {
		resp.QRExpiresAt = inst.QRExpiresAt.Time
	}
	resp.CreatedAt = inst.CreatedAt
	if inst.ConnectedAt.Valid {
		resp.ConnectedAt = inst.ConnectedAt.Time
	}
	if inst.DisconnectedAt.Valid {
		resp.DisconnectedAt = inst.DisconnectedAt.Time
	}
	if inst.LastSeen.Valid {
		resp.LastSeen = inst.LastSeen.Time
	}

	if inst.Keterangan.Valid {
		resp.Keterangan = inst.Keterangan.String
	}

	resp.Used = inst.Used
	if inst.Keterangan.Valid {
		resp.Keterangan = inst.Keterangan.String
	}

	if inst.CreatedBy.Valid {
		resp.CreatedBy = inst.CreatedBy.Int64
	}

	return resp
}

// UpdateInstanceFieldsRequest for PATCH /instances/:instanceId
type UpdateInstanceFieldsRequest struct {
	Used       *bool   `json:"used"`       // pointer to allow null (optional)
	Keterangan *string `json:"keterangan"` // pointer to allow null (optional)
	Circle     *string `json:"circle"`     // pointer to allow null (optional)
}

// UpdateInstanceFields updates used and keterangan fields
func UpdateInstanceFields(instanceID string, req *UpdateInstanceFieldsRequest) error {
	// Build dynamic query based on what fields are provided
	query := "UPDATE instances SET "
	args := []interface{}{}
	argCount := 1
	updates := []string{}

	if req.Used != nil {
		updates = append(updates, fmt.Sprintf("used = $%d", argCount))
		args = append(args, *req.Used)
		argCount++
	}

	if req.Keterangan != nil {
		updates = append(updates, fmt.Sprintf("keterangan = $%d", argCount))
		args = append(args, *req.Keterangan)
		argCount++
	}

	if req.Circle != nil {
		updates = append(updates, fmt.Sprintf("circle = $%d", argCount))
		args = append(args, *req.Circle)
		argCount++
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	query += updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}

	query += fmt.Sprintf(" WHERE instance_id = $%d", argCount)
	args = append(args, instanceID)

	result, err := database.AppDB.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// TimelineEvent represents a single event in instance timeline
type TimelineEvent struct {
	EventDate   time.Time `json:"eventDate"`
	EventType   string    `json:"eventType"`
	Description string    `json:"description"`
	Metadata    string    `json:"metadata,omitempty"`
	UserID      *int64    `json:"userId,omitempty"`
}

// GetInstanceTimeline retrieves timeline events for an instance from multiple tables
func GetInstanceTimeline(instanceID string, startDate, endDate string) ([]TimelineEvent, error) {
	query := `
		-- Instance creation
		SELECT created_at as event_date, 'INSTANCE_CREATED' as event_type, 
		       'Instance created' as description, NULL as metadata, created_by as user_id
		FROM instances 
		WHERE instance_id = $1

		UNION ALL

		-- User assignments
		SELECT ui.created_at, 'USER_ASSIGNED',
		       'User assigned to instance' as description,
		       NULL as metadata, ui.user_id
		FROM user_instances ui
		WHERE ui.instance_id = $1

		UNION ALL

		-- Warming rooms (as sender)
		SELECT wr.created_at, 'WARMING_ROOM_JOINED',
		       'Joined warming room: ' || wr.name || ' (as sender)' as description,
		       json_build_object('roomId', wr.id, 'roomName', wr.name, 'role', 'sender', 'roomType', wr.room_type)::text as metadata,
		       NULL as user_id
		FROM warming_rooms wr
		WHERE wr.sender_instance_id = $1

		UNION ALL

		-- Warming rooms (as receiver)
		SELECT wr.created_at, 'WARMING_ROOM_JOINED',
		       'Joined warming room: ' || wr.name || ' (as receiver)' as description,
		       json_build_object('roomId', wr.id, 'roomName', wr.name, 'role', 'receiver', 'roomType', wr.room_type)::text as metadata,
		       NULL as user_id
		FROM warming_rooms wr
		WHERE wr.receiver_instance_id = $1

		UNION ALL

		-- Warming activities
		SELECT wl.executed_at, 'WARMING_ACTIVITY',
		       'Warming message in room: ' || wr.name as description,
		       json_build_object('roomId', wr.id, 'roomName', wr.name, 'status', wl.status)::text as metadata,
		       NULL as user_id
		FROM warming_logs wl
		JOIN warming_rooms wr ON wl.room_id = wr.id
		WHERE wr.sender_instance_id = $1 OR wr.receiver_instance_id = $1

		UNION ALL

		-- Worker configurations (by circle)
		SELECT wc.created_at, 'WORKER_CREATED',
		       'Worker "' || wc.worker_name || '" created for circle: ' || wc.circle as description,
		       json_build_object('workerId', wc.id, 'workerName', wc.worker_name, 'circle', wc.circle, 'application', wc.application, 'enabled', wc.enabled)::text as metadata,
		       wc.user_id
		FROM outbox_worker_config wc
		JOIN instances i ON i.circle = wc.circle
		WHERE i.instance_id = $1

		UNION ALL

		-- Audit logs
		SELECT al.created_at, 'AUDIT_' || UPPER(al.action),
		       al.action || ' on instance' as description,
		       al.details::text as metadata,
		       al.user_id
		FROM audit_logs al
		WHERE al.resource_type = 'instance' AND al.resource_id = $1

		UNION ALL

		-- Message Stats
		SELECT stat_date::timestamp with time zone as event_date, 'MESSAGE_STATS',
		       'Messages sent: ' || message_count as description,
		       json_build_object('count', message_count)::text as metadata,
		       NULL as user_id
		FROM instance_message_stats
		WHERE instance_id = $1

		ORDER BY event_date DESC
		LIMIT 500
	`

	var events []TimelineEvent
	rows, err := database.AppDB.Query(query, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var event TimelineEvent
		var metadata sql.NullString
		var userID sql.NullInt64

		err := rows.Scan(&event.EventDate, &event.EventType, &event.Description, &metadata, &userID)
		if err != nil {
			return nil, err
		}

		if metadata.Valid {
			event.Metadata = metadata.String
		}
		if userID.Valid {
			event.UserID = &userID.Int64
		}

		events = append(events, event)
	}

	return events, nil
}

// GetGlobalTimeline retrieves timeline events for all instances of a user plus attendance
// Admin role sees all data, regular user only sees their own.
func GetGlobalTimeline(userID int64, role string, startDate, endDate string) ([]TimelineEvent, error) {
	isAdmin := role == "admin"

	query := `
		WITH user_instance_ids AS (
			SELECT instance_id FROM user_instances WHERE user_id = $1 OR $2 = true
		)
		-- Instance lifecycle
		SELECT i.created_at as event_date, 'INSTANCE_CREATED' as event_type, 
		       'Instance created: ' || i.instance_id as description, 
		       json_build_object('instanceId', i.instance_id)::text as metadata, 
		       i.created_by as user_id
		FROM instances i
		WHERE $2 = true OR i.instance_id IN (SELECT instance_id FROM user_instance_ids)

		UNION ALL

		-- Warming rooms (participation)
		SELECT wr.created_at, 'WARMING_ROOM_JOINED',
		       'Joined warming room: ' || wr.name || ' (Participation)' as description,
		       json_build_object('roomId', wr.id, 'roomName', wr.name, 'roomType', wr.room_type)::text as metadata,
		       NULL as user_id
		FROM warming_rooms wr
		WHERE $2 = true 
		   OR wr.sender_instance_id IN (SELECT instance_id FROM user_instance_ids)
		   OR wr.receiver_instance_id IN (SELECT instance_id FROM user_instance_ids)

		UNION ALL

		-- Warming activities
		SELECT wl.executed_at, 'WARMING_ACTIVITY',
		       'Warming message in room: ' || wr.name as description,
		       json_build_object('roomId', wr.id, 'roomName', wr.name, 'status', wl.status)::text as metadata,
		       NULL as user_id
		FROM warming_logs wl
		JOIN warming_rooms wr ON wl.room_id = wr.id
		WHERE $2 = true 
		   OR wl.sender_instance_id IN (SELECT instance_id FROM user_instance_ids)
		   OR wl.receiver_instance_id IN (SELECT instance_id FROM user_instance_ids)

		UNION ALL

		-- SIM Attendance
		SELECT attendance_date::timestamp with time zone, 'SIM_ATTENDANCE',
		       title as description,
		       json_build_object('simLabel', sim_label, 'phoneNumber', phone_number, 'status', status, 'notes', notes)::text as metadata,
		       user_id
		FROM sim_attendance
		WHERE $2 = true OR user_id = $1

		UNION ALL

		-- Audit logs
		SELECT al.created_at, 'AUDIT_' || UPPER(al.action),
		       al.action || ' on ' || al.resource_type as description,
		       al.details::text as metadata,
		       al.user_id
		FROM audit_logs al
		WHERE $2 = true OR al.user_id = $1

		UNION ALL

		-- Message Stats
		SELECT ms.stat_date::timestamp with time zone as event_date, 'MESSAGE_STATS',
		       'Messages sent: ' || ms.message_count || ' (Instance: ' || ms.instance_id || ')' as description,
		       json_build_object('instanceId', ms.instance_id, 'count', ms.message_count)::text as metadata,
		       NULL as user_id
		FROM instance_message_stats ms
		WHERE $2 = true OR ms.instance_id IN (SELECT instance_id FROM user_instance_ids)

		ORDER BY event_date DESC
		LIMIT 1000
	`

	var events []TimelineEvent
	rows, err := database.AppDB.Query(query, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var event TimelineEvent
		var metadata sql.NullString
		var uID sql.NullInt64

		err := rows.Scan(&event.EventDate, &event.EventType, &event.Description, &metadata, &uID)
		if err != nil {
			return nil, err
		}

		if metadata.Valid {
			event.Metadata = metadata.String
		}
		if uID.Valid {
			event.UserID = &uID.Int64
		}

		events = append(events, event)
	}

	return events, nil
}
