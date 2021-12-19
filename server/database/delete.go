package database

func (db *Database) Delete(containerID string) error {
	if _, err := db.Exec("DELETE FROM auth WHERE container_id=$1", containerID); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM settings WHERE container_id=$1", containerID); err != nil {
		return err
	}
	return nil
}
