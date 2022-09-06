package collector

import (
  "time"
)

type InstanceAction struct {
  Action string    `json:"action"`
  Time   time.Time `json:"time"`
}

type ScheduledEvent struct {
  State       string    `json:"State"`
  Code        string    `json:"Code"`
  Description string    `json:"Description"`
  NotBefore   time.Time `json:"NotBefore"`
  NotAfter    time.Time `json:"NotAfter"`
}
