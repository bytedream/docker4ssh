package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Settings is the raw version of docker.Config
type Settings struct {
	NetworkMode        *int    `json:"network_mode"`
	Configurable       *bool   `json:"configurable"`
	RunLevel           *int    `json:"run_level"`
	StartupInformation *bool   `json:"startup_information"`
	ExitAfter          *string `json:"exit_after"`
	KeepOnExit         *bool   `json:"keep_on_exit"`
}

func (db *Database) SettingsByContainerID(containerID string) (Settings, error) {
	row := db.QueryRow("SELECT network_mode, configurable, run_level, startup_information, exit_after, keep_on_exit FROM settings WHERE container_id LIKE $1", fmt.Sprintf("%s%%", containerID))

	var settings Settings

	if err := row.Scan(&settings.NetworkMode, &settings.Configurable, &settings.RunLevel, &settings.StartupInformation, &settings.ExitAfter, &settings.KeepOnExit); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func (db *Database) SetSettings(containerID string, settings Settings) error {
	query := make(map[string]interface{}, 0)

	body, _ := json.Marshal(settings)
	json.Unmarshal(body, &query)

	var keys, values []string
	for k, v := range query {
		if v != nil {
			keys = append(keys, k)
			switch reflect.ValueOf(v).Kind() {
			case reflect.String:
				values = append(values, fmt.Sprintf("\"%v\"", v))
			case reflect.Bool:
				if v.(bool) {
					values = append(values, fmt.Sprintf("%v", 1))
				} else {
					values = append(values, fmt.Sprintf("%v", 0))
				}
			default:
				values = append(values, fmt.Sprintf("%v", v))
			}
		}
	}

	err := db.QueryRow("SELECT 1 FROM settings WHERE container_id=$1", containerID).Scan()
	if err == sql.ErrNoRows {
		keys = append(keys, "container_id")
		values = append(values, fmt.Sprintf("\"%s\"", containerID))

		_, err = db.Exec(fmt.Sprintf("INSERT INTO settings (%s) VALUES (%s)", strings.Join(keys, ", "), strings.Join(values, ", ")))
	} else if len(keys) > 0 {
		var set []string

		for i := 0; i < len(keys); i++ {
			set = append(set, fmt.Sprintf("%s=%s", keys[i], values[i]))
		}

		_, err = db.Exec(fmt.Sprintf("UPDATE settings SET %s WHERE container_id=$1", strings.Join(set, ", ")), containerID)
	} else {
		err = nil
	}
	return err
}
