package handler

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"gowa-yourself/internal/helper"
	"gowa-yourself/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ContactInfo represents contact information for list and export
type ContactInfo struct {
	JID         string `json:"jid"`
	PhoneNumber string `json:"phoneNumber"`
	Name        string `json:"name"`
	IsGroup     bool   `json:"isGroup"`
	IsLID       bool   `json:"isLID,omitempty"`
}

// CheckNumberRequest for checking if phone number is registered
type CheckNumberRequest struct {
	Phone string `json:"phone" validate:"required"`
}

// POST /check/:instanceId
func CheckNumber(c echo.Context) error {
	instanceID := c.Param("instanceId")

	var req CheckNumberRequest
	if err := c.Bind(&req); err != nil {
		return ErrorResponse(c, 400, "Invalid request body", "INVALID_REQUEST", err.Error())
	}

	session, err := service.GetSession(instanceID)
	if err != nil {
		return ErrorResponse(c, 404, "Session not found", "SESSION_NOT_FOUND", "")
	}

	if !session.Client.IsConnected() {
		return ErrorResponse(c, 400, "Session is not connected", "NOT_CONNECTED", "")
	}

	// Import helper package for phone number formatting
	recipient, err := helper.FormatPhoneNumber(req.Phone)
	if err != nil {
		return ErrorResponse(c, 400, "Invalid phone number", "INVALID_PHONE", err.Error())
	}

	willSkipValidation := helper.ShouldSkipValidation(req.Phone)

	isRegistered, err := session.Client.IsOnWhatsApp(context.Background(), []string{recipient.User})
	if err != nil {
		return ErrorResponse(c, 500, "Failed to check phone number", "CHECK_FAILED", err.Error())
	}

	if len(isRegistered) == 0 {
		return ErrorResponse(c, 400, "Unable to verify number", "VERIFICATION_ERROR", "")
	}

	return SuccessResponse(c, 200, "Phone number checked", map[string]interface{}{
		"phone":              req.Phone,
		"isRegistered":       isRegistered[0].IsIn,
		"jid":                isRegistered[0].JID.String(),
		"willSkipValidation": willSkipValidation,
		"note":               getValidationNote(isRegistered[0].IsIn, willSkipValidation),
	})
}

// Helper function to provide user-friendly note about validation behavior
func getValidationNote(isRegistered, willSkip bool) string {
	if willSkip {
		if isRegistered {
			return "Number is registered. Validation will be skipped when sending (ALLOW_9_DIGIT_PHONE_NUMBER=true)"
		}
		return "Number appears unregistered, but validation will be skipped when sending (ALLOW_9_DIGIT_PHONE_NUMBER=true). Message will be attempted anyway."
	}
	if isRegistered {
		return "Number is registered and will pass validation when sending"
	}
	return "Number is not registered. Message sending will be blocked unless ALLOW_9_DIGIT_PHONE_NUMBER=true is set"
}

// GET /contacts/:instanceId/:jid
func GetContactDetail(c echo.Context) error {
	instanceID := c.Param("instanceId")
	jidParam := c.Param("jid")

	session, err := service.GetSession(instanceID)
	if err != nil {
		return ErrorResponse(c, 404, "Session not found", "SESSION_NOT_FOUND", "Please login first")
	}

	if !session.IsConnected {
		return ErrorResponse(c, 400, "Session is not connected", "NOT_CONNECTED", "Please check /status endpoint")
	}

	if !session.Client.IsConnected() {
		return ErrorResponse(c, 400, "WhatsApp connection lost", "CONNECTION_LOST", "Please reconnect")
	}

	if session.Client.Store.ID == nil {
		return ErrorResponse(c, 400, "Not logged in", "NOT_LOGGED_IN", "Please scan QR code first")
	}

	// Parse JID
	jid, err := types.ParseJID(jidParam)
	if err != nil {
		return ErrorResponse(c, 400, "Invalid JID format", "INVALID_JID", err.Error())
	}

	// Get contact from store
	contact, err := session.Client.Store.Contacts.GetContact(context.Background(), jid)
	if err != nil {
		return ErrorResponse(c, 404, "Contact not found", "CONTACT_NOT_FOUND", err.Error())
	}

	// Build contact detail
	contactDetail := map[string]interface{}{
		"jid":            jid.String(),
		"phoneNumber":    jid.User,
		"name":           contact.FullName,
		"businessName":   contact.BusinessName,
		"pushName":       contact.PushName,
		"profilePicture": "",
		"about":          "",
		"isGroup":        jid.Server == "g.us",
		"isBusiness":     contact.BusinessName != "",
		"verifiedName":   nil,
	}

	// Set default name if empty
	if contactDetail["name"] == "" {
		if contact.BusinessName != "" {
			contactDetail["name"] = contact.BusinessName
		} else if contact.PushName != "" {
			contactDetail["name"] = contact.PushName
		} else {
			contactDetail["name"] = jid.User
		}
	}

	// Fetch profile picture
	pic, err := session.Client.GetProfilePictureInfo(context.Background(), jid, &whatsmeow.GetProfilePictureParams{
		Preview: false,
	})
	if err == nil && pic != nil {
		contactDetail["profilePicture"] = pic.URL
	}

	// Fetch user info (about status) - only for non-groups
	if jid.Server != "g.us" {
		userInfo, err := session.Client.GetUserInfo(context.Background(), []types.JID{jid})
		if err == nil && len(userInfo) > 0 {
			info := userInfo[jid]

			// About/Status
			if info.Status != "" {
				contactDetail["about"] = info.Status
			}

			// Business info
			if info.VerifiedName != nil && info.VerifiedName.Details.GetVerifiedName() != "" {
				contactDetail["isBusiness"] = true
				contactDetail["verifiedName"] = info.VerifiedName.Details.GetVerifiedName()
			}
		}
	}

	return SuccessResponse(c, 200, "Contact details retrieved successfully", contactDetail)
}

// GET /contacts/:instanceId?page=1&limit=50&search=john
func GetContactList(c echo.Context) error {
	instanceID := c.Param("instanceId")

	session, err := service.GetSession(instanceID)
	if err != nil {
		return ErrorResponse(c, 404, "Session not found", "SESSION_NOT_FOUND", "Please login first")
	}

	if !session.IsConnected {
		return ErrorResponse(c, 400, "Session is not connected", "NOT_CONNECTED", "Please check /status endpoint")
	}

	if !session.Client.IsConnected() {
		return ErrorResponse(c, 400, "WhatsApp connection lost", "CONNECTION_LOST", "Please reconnect")
	}

	if session.Client.Store.ID == nil {
		return ErrorResponse(c, 400, "Not logged in", "NOT_LOGGED_IN", "Please scan QR code first")
	}

	// Parse pagination params (default: page=1, limit=50, max=50)
	page := 1
	limit := 50
	searchQuery := strings.ToLower(strings.TrimSpace(c.QueryParam("search")))

	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	contacts, err := session.Client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return ErrorResponse(c, 500, "Failed to retrieve contact list", "FETCH_FAILED", err.Error())
	}

	// Build contact list with name fallback and LID resolution
	contactMap := make(map[string]ContactInfo)
	for jid, contact := range contacts {
		contactInfo := ContactInfo{
			JID:         jid.String(),
			PhoneNumber: jid.User,
			Name:        contact.FullName,
			IsGroup:     jid.Server == "g.us",
			IsLID:       jid.Server == "lid",
		}

		// Skip all LID contacts (linked devices)
		if jid.Server == "lid" {
			continue
		}

		if contactInfo.Name == "" {
			if contact.BusinessName != "" {
				contactInfo.Name = contact.BusinessName
			} else if contact.PushName != "" {
				contactInfo.Name = contact.PushName
			} else {
				contactInfo.Name = contactInfo.PhoneNumber
			}
		}

		// Deduplicate: group by phone number, prefer @s.whatsapp.net over @lid
		key := contactInfo.PhoneNumber
		if existing, exists := contactMap[key]; exists {
			// Keep @s.whatsapp.net over @lid
			if existing.IsLID && !contactInfo.IsLID {
				contactMap[key] = contactInfo
			}
			// If both are same type, keep the one with better name
			if existing.IsLID == contactInfo.IsLID && contactInfo.Name != contactInfo.PhoneNumber {
				contactMap[key] = contactInfo
			}
		} else {
			contactMap[key] = contactInfo
		}
	}

	// Convert map to slice and apply search filter
	allContacts := make([]ContactInfo, 0, len(contactMap))
	for _, contactInfo := range contactMap {
		// Filter by search query (case-insensitive)
		if searchQuery != "" {
			nameMatch := strings.Contains(strings.ToLower(contactInfo.Name), searchQuery)
			jidMatch := strings.Contains(strings.ToLower(contactInfo.JID), searchQuery)
			phoneMatch := strings.Contains(strings.ToLower(contactInfo.PhoneNumber), searchQuery)
			if !nameMatch && !jidMatch && !phoneMatch {
				continue
			}
		}

		allContacts = append(allContacts, contactInfo)
	}

	// Calculate pagination
	totalContacts := len(allContacts)
	totalPages := (totalContacts + limit - 1) / limit

	startIndex := (page - 1) * limit
	endIndex := startIndex + limit

	// Handle out of range page
	if startIndex >= totalContacts {
		return SuccessResponse(c, 200, "Contact list retrieved successfully", map[string]interface{}{
			"total":       totalContacts,
			"page":        page,
			"limit":       limit,
			"totalPages":  totalPages,
			"search":      searchQuery,
			"contacts":    []ContactInfo{},
			"hasNextPage": false,
			"hasPrevPage": page > 1,
		})
	}

	if endIndex > totalContacts {
		endIndex = totalContacts
	}

	paginatedContacts := allContacts[startIndex:endIndex]

	return SuccessResponse(c, 200, "Contact list retrieved successfully", map[string]interface{}{
		"total":       totalContacts,
		"page":        page,
		"limit":       limit,
		"totalPages":  totalPages,
		"search":      searchQuery,
		"contacts":    paginatedContacts,
		"hasNextPage": page < totalPages,
		"hasPrevPage": page > 1,
	})
}

// GET /contacts/:instanceId/export?format=xlsx
func ExportContacts(c echo.Context) error {
	instanceID := c.Param("instanceId")
	format := c.QueryParam("format")

	// Default to xlsx if not specified
	if format == "" {
		format = "xlsx"
	}

	// Validate format
	if format != "xlsx" && format != "csv" {
		return ErrorResponse(c, 400, "Invalid format", "INVALID_FORMAT", "Format must be 'xlsx' or 'csv'")
	}

	session, err := service.GetSession(instanceID)
	if err != nil {
		return ErrorResponse(c, 404, "Session not found", "SESSION_NOT_FOUND", "Please login first")
	}

	if !session.IsConnected {
		return ErrorResponse(c, 400, "Session is not connected", "NOT_CONNECTED", "Please check /status endpoint")
	}

	if !session.Client.IsConnected() {
		return ErrorResponse(c, 400, "WhatsApp connection lost", "CONNECTION_LOST", "Please reconnect")
	}

	if session.Client.Store.ID == nil {
		return ErrorResponse(c, 400, "Not logged in", "NOT_LOGGED_IN", "Please scan QR code first")
	}

	// Get all contacts
	contacts, err := session.Client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return ErrorResponse(c, 500, "Failed to retrieve contact list", "FETCH_FAILED", err.Error())
	}

	// Build contact list with deduplication
	contactMap := make(map[string]ContactInfo)
	for jid, contact := range contacts {
		// Skip LID contacts
		if jid.Server == "lid" {
			continue
		}

		name := contact.FullName
		if name == "" {
			if contact.BusinessName != "" {
				name = contact.BusinessName
			} else if contact.PushName != "" {
				name = contact.PushName
			} else {
				name = jid.User
			}
		}

		contactInfo := ContactInfo{
			PhoneNumber: jid.User,
			Name:        name,
			JID:         jid.String(),
			IsGroup:     jid.Server == "g.us",
		}

		// Deduplicate by phone number
		key := contactInfo.PhoneNumber
		if existing, exists := contactMap[key]; exists {
			// Prefer non-group over group, or better name
			if !existing.IsGroup || contactInfo.Name != contactInfo.PhoneNumber {
				contactMap[key] = contactInfo
			}
		} else {
			contactMap[key] = contactInfo
		}
	}

	// Convert to slice
	allContacts := make([]ContactInfo, 0, len(contactMap))
	for _, contact := range contactMap {
		allContacts = append(allContacts, contact)
	}

	// Export based on format
	if format == "xlsx" {
		return exportToExcel(c, allContacts, instanceID)
	}
	return exportToCSV(c, allContacts, instanceID)
}

func exportToExcel(c echo.Context, contacts []ContactInfo, instanceID string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Contacts"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return ErrorResponse(c, 500, "Failed to create Excel sheet", "EXCEL_ERROR", err.Error())
	}

	// Set headers
	headers := []string{"No", "Phone Number", "Name", "JID", "Type"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetCellStyle(sheetName, "A1", "E1", headerStyle)

	// Add data
	for i, contact := range contacts {
		row := i + 2
		contactType := "Contact"
		if contact.IsGroup {
			contactType = "Group"
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), i+1)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), contact.PhoneNumber)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), contact.Name)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), contact.JID)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), contactType)
	}

	// Auto-fit columns
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 15)
	f.SetColWidth(sheetName, "C", "C", 25)
	f.SetColWidth(sheetName, "D", "D", 35)
	f.SetColWidth(sheetName, "E", "E", 10)

	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Set response headers
	filename := fmt.Sprintf("contacts_%s.xlsx", instanceID)
	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return f.Write(c.Response().Writer)
}

func exportToCSV(c echo.Context, contacts []ContactInfo, instanceID string) error {
	c.Response().Header().Set("Content-Type", "text/csv")
	filename := fmt.Sprintf("contacts_%s.csv", instanceID)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Response().Writer)
	defer writer.Flush()

	// Write headers
	headers := []string{"No", "Phone Number", "Name", "JID", "Type"}
	if err := writer.Write(headers); err != nil {
		return ErrorResponse(c, 500, "Failed to write CSV headers", "CSV_ERROR", err.Error())
	}

	// Write data
	for i, contact := range contacts {
		contactType := "Contact"
		if contact.IsGroup {
			contactType = "Group"
		}

		row := []string{
			strconv.Itoa(i + 1),
			contact.PhoneNumber,
			contact.Name,
			contact.JID,
			contactType,
		}

		if err := writer.Write(row); err != nil {
			return ErrorResponse(c, 500, "Failed to write CSV row", "CSV_ERROR", err.Error())
		}
	}

	return nil
}
