package core

import (
	"fmt"
	"time"

	"github.com/asdine/storm"
	"github.com/opsgenie/opsgenie-go-sdk/alertsv2"
	"github.com/opsgenie/opsgenie-go-sdk/client"
)

var alertCli *client.OpsGenieAlertV2Client

// SetOpsGenieToken sets the api key and initialises an OpsGenie alert client
func SetOpsGenieToken(apiKey string) error {
	cli := &client.OpsGenieClient{}
	cli.SetAPIKey(apiKey)
	var err error
	alertCli, err = cli.AlertV2()
	return err
}

// NewAlert returns a new Alert
func NewAlert(name, resourceID string) *Alert {
	return &Alert{
		ID:          fmt.Sprintf("aunt.%s.%s", name, resourceID),
		Entity:      resourceID,
		Details:     make(map[string]string),
		LastUpdated: time.Now(),
	}
}

// Purge will remove and close alerts that no longer is alerted
func Purge(db *storm.DB, olderThan time.Duration) error {
	var resources []*Alert

	startTime := time.Time{}
	endTime := time.Now().Add(-1 * olderThan)

	if err := db.Range("LastUpdated", startTime, endTime, &resources); err != nil && err != storm.ErrNotFound {
		return err
	}

	for _, i := range resources {
		if err := i.Delete(db); err != nil {
			fmt.Printf("alert purge error: %v %s\n", err, i.ID)
		}
	}
	return nil
}

// Alert is an app specific representation of a OPSGenie alert
type Alert struct {
	// ID, The unique identifier for this alert
	ID string
	// Alert text, should not be more than 130 characters long
	Message string
	// The name of the entity that the alert is related to. For example, name of the application, server etc.
	Entity string
	// The source of the alert
	Source string
	// Detailed description of the alert; anything that may not have fit in the Message field can be entered here.
	Description string
	// User defined properties of this alert, e.g. IP addresses, limits, accounts and regions
	Details map[string]string
	// LastUpdated represents the last time this alert was raised
	LastUpdated time.Time
}

// Returns a string representation of this alert
func (a *Alert) String() string {
	return fmt.Sprintf("%s (%s), %s", a.Message, a.Entity, a.Details)
}

// Save this Alert to the database and sends an alert to OpsGenie
func (a *Alert) Save(db *storm.DB) error {
	fmt.Printf("Creating: %s\n", a)
	if err := db.Save(a); err != nil {
		return err
	}
	if alertCli == nil {
		return nil
	}

	if len(a.Description) > 5000 {
		a.Description = a.Description[0:4999]
	}
	_, err := alertCli.Create(alertsv2.CreateAlertRequest{
		Message:     a.Message,
		Alias:       a.ID,
		Details:     a.Details,
		Description: a.Description,
		Entity:      a.Entity,
		Source:      "aunt ",
		Priority:    alertsv2.P2,
		Tags:        []string{"SSP"},
	})
	return err
}

// Delete this Alert and close the OpsGenie alert
func (a *Alert) Delete(db *storm.DB) error {
	fmt.Printf("Closing: %s\n", a)
	if err := db.DeleteStruct(a); err != nil {
		return err
	}
	if alertCli == nil {
		return nil
	}
	_, err := alertCli.Close(alertsv2.CloseRequest{
		Identifier: &alertsv2.Identifier{
			Alias: a.ID,
		},
	})
	return err
}
