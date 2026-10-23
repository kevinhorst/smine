package server

import (
	"github.com/kevinhorst/smine/internal/heartbeat"
)

// featureEvent is one change of a session feature, its time in the server's zone.
type featureEvent struct {
	Action string
	At     string
}

// featureRow is one switchable feature of a session with its record history; a hint means it warns.
type featureRow struct {
	Events  []featureEvent
	Feature string
	Hint    string
	Hook    string
	State   string
	Tags    []mechanismTag
}

type fleetPage struct {
	Fleet string
	Page  string
	Title string
}

type fleetStatusPage struct {
	ArchiveErr string
	Archived   []heartbeat.ArchivedFleet
	Document   *heartbeat.Document
	Filter     string
	Fleets     []fleetView
	Problems   []fleetProblem
	Queue      []heartbeat.QueueItem
	Sessions   []heartbeat.Session
	StatusErr  string
}

type fleetProblem struct {
	Details []string
	Summary string
}

type fleetArchivePage struct {
	Archived *heartbeat.ArchivedFleet
	Page     string
	Title    string
}

type fleetView struct {
	Fleet   heartbeat.Fleet
	Members []heartbeat.MemberRow
	Train   []heartbeat.TrainStep
}

type runtimePage struct {
	Page  string
	Title string
}

type runtimeStatusPage struct {
	Document  *heartbeat.RuntimeDocument
	StatusErr string
}

type titleView struct {
	Badge   string
	Meaning string
	Skill   string
	State   string
	Text    string
}
