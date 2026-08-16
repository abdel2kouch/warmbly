package campaign

import (
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

func TestCampaignWindowOpenNow_AllDayEveryDay(t *testing.T) {
	var windows models.ScheduleWindows
	for day := range windows {
		windows[day] = []models.TimeInterval{{Start: 0, End: 1440}}
	}
	campaign := &models.Campaign{Timezone: "America/New_York", ScheduleWindows: windows}
	now := time.Date(2026, time.August, 17, 3, 15, 0, 0, time.UTC)
	if !campaignWindowOpenNow(campaign, now) {
		t.Fatal("expected a 24/7 campaign to allow an immediate wakeup")
	}
}

func TestCampaignWindowOpenNow_OutsideWindow(t *testing.T) {
	var windows models.ScheduleWindows
	windows[time.Monday] = []models.TimeInterval{{Start: 9 * 60, End: 17 * 60}}
	campaign := &models.Campaign{Timezone: "UTC", ScheduleWindows: windows}
	now := time.Date(2026, time.August, 17, 8, 30, 0, 0, time.UTC) // Monday
	if campaignWindowOpenNow(campaign, now) {
		t.Fatal("expected a campaign outside its window to retain the scheduled wakeup")
	}
}

func TestCampaignWindowOpenNow_RespectsFutureStartDate(t *testing.T) {
	var windows models.ScheduleWindows
	for day := range windows {
		windows[day] = []models.TimeInterval{{Start: 0, End: 1440}}
	}
	now := time.Date(2026, time.August, 17, 8, 30, 0, 0, time.UTC)
	start := now.Add(time.Hour)
	campaign := &models.Campaign{Timezone: "UTC", ScheduleWindows: windows, StartDate: &start}
	if campaignWindowOpenNow(campaign, now) {
		t.Fatal("expected a future campaign start date to prevent an immediate wakeup")
	}
}
