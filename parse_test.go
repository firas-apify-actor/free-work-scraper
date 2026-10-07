package main

import "testing"

func TestParseRate(t *testing.T) {
	cases := []struct {
		in     string
		lo, hi float64 // 0 = expect nil
	}{
		{"450-550 €/j", 450, 550},
		{"500€ HT/jour", 500, 500},
		{"TJM: 600", 600, 600},
		{"TJM 450 - 550 €", 450, 550},
		{"de 400 à 500 euros par jour", 400, 500},
		{"600€/j HT", 600, 600},
		{"500 € / jour", 500, 500},
		{"TJM cible : 650€", 650, 650},
		{"TJM max 700 euros", 700, 700},
		{"Tarif 550 euros par jour, démarrage ASAP", 550, 550},
		{"500–600 €/jour selon profil", 500, 600},
		{"TJM=480", 480, 480},
		{"Budget : 700/j", 700, 700},
		{"600-450 €/j", 450, 600}, // reversed range
		{"Salaire 45k€ à 55k€ par an", 0, 0},
		{"TJM à négocier", 0, 0},
		{"TJM: selon profil", 0, 0},
		{"Mission de 6 mois, 3 jours par semaine", 0, 0},
		{"Rémunération : 55000 € / an", 0, 0},
		{"TJM: 50", 0, 0},   // implausibly low
		{"TJM: 9000", 0, 0}, // implausibly high
		{"", 0, 0},
	}
	for _, c := range cases {
		lo, hi := parseRate(c.in)
		if c.lo == 0 {
			if lo != nil || hi != nil {
				t.Errorf("%q: expected nil, got %v-%v", c.in, *lo, *hi)
			}
			continue
		}
		if lo == nil || hi == nil || *lo != c.lo || *hi != c.hi {
			t.Errorf("%q: want %v-%v, got %v %v", c.in, c.lo, c.hi, lo, hi)
		}
	}
}

func TestMissionNormalization(t *testing.T) {
	r := rawJob{ID: 1, Title: "t", Slug: "s", PublishedAt: "2026-10-01T10:00:00+02:00",
		Description: "<p>TJM : 520</p>", StartsAt: "2027-01-04T00:00:00+01:00", ExperienceLevel: "weird"}
	m := r.mission(false)
	if m.DailyRateMin == nil || *m.DailyRateMin != 520 || m.RemoteMode != "unknown" ||
		m.ExperienceLevel != "unknown" || m.StartDate != "2027-01-04" || m.Description != "" {
		t.Fatalf("%+v", m)
	}
	two := 400.0
	r.MinDailySalary = &two // structured API value wins over the text
	if m := r.mission(false); *m.DailyRateMin != 400 {
		t.Fatal("structured rate must win")
	}
}
