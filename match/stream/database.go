package stream

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	dbhost     = os.Getenv("DB_HOST")
	dbport     = os.Getenv("DB_PORT")
	dbuser     = os.Getenv("DB_USER")
	dbpassword = os.Getenv("DB_PASS")
	dbname     = os.Getenv("DB_NAME")
	psqlconn   = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbhost, dbport, dbuser, dbpassword, dbname)
	// urlExample := "postgres://username:password@localhost:5432/database_name"
	psqlurl = fmt.Sprintf("postgres://%s:%s@%s:%s/%s", dbuser, dbpassword, dbhost, dbport, dbname)
)

type Database struct {
}

// TODO PRIO: migrate to pgxw

func (d *Database) save_room_id_with_ip(room_id string, ip string) error {
	conn, err := pgxw.Connect(psqlurl)
	if err != nil {
		err = errors.New("Postgres connection error: " + err.Error())
		log.Println(err.Error())
		return err
	}
	defer conn.Close()

	_, err = conn.Exec("INSERT INTO rooms(room_id, task_private_ip) VALUES($1, $2);", room_id, ip)
	if err != nil {
		err = errors.New("Postgres exec error: " + err.Error())
		log.Println(err.Error())
		return err
	}
	return nil
}

func (d *Database) remove_room_id(room_id string) error {
	conn, err := pgxw.Connect(psqlurl)
	if err != nil {
		err = errors.New("Postgres connection error: " + err.Error())
		log.Println(err.Error())
		return err
	}
	defer conn.Close()

	_, err = conn.Exec("DELETE FROM rooms WHERE room_id=$1;", room_id)
	if err != nil {
		err = errors.New("Postgres exec error: " + err.Error())
		log.Println(err.Error())
		return err
	}
	return nil
}

func (d *Database) fetch_ip_from_room_id(room_id string) (string, bool, error) {
	// bool marks if the error is fatal
	conn, err := pgxw.Connect(psqlurl)
	if err != nil {
		err = errors.New("Postgres connection error: " + err.Error())
		log.Println(err.Error())
		return "", true, err
	}
	defer conn.Close()

	var ip string
	err = conn.QueryRow("SELECT task_private_ip FROM rooms WHERE room_id=$1;", room_id).Scan(&ip)
	switch err {
	case sql.ErrNoRows:
		err = errors.New(fmt.Sprintf("IP of room_id: %s not found. %s", room_id, err.Error()))
		log.Println(err.Error())
		return "", false, err
	case nil:
		return ip, false, nil
	default:
		err = errors.New("Postgres query unexpected error: " + err.Error())
		log.Println(err.Error())
		return "", true, err
	}
}

func (d *Database) fetch_unique_room_id() (string, error) {
	conn, err := pgxw.Connect(psqlurl)
	if err != nil {
		err = errors.New("Postgres connection error: " + err.Error())
		log.Println(err.Error())
		return "", err
	}
	defer conn.Close()

	var room_id string
	err = conn.QueryRow("SELECT nextval('room_id_seq');").Scan(&room_id)
	if err != nil {
		err = errors.New("Postgres query error: " + err.Error())
		log.Println(err.Error())
		return "", err
	}
	log.Println("room_id:", room_id)
	return room_id, nil
}

func (d *Database) health_database() (string, error) {
	conn, err := pgxw.Connect(psqlurl)
	if err != nil {
		err = errors.New("Postgres connection error: " + err.Error())
		log.Println(err.Error())
		return "", err
	}
	defer conn.Close()

	rows, err := conn.Query("SELECT * FROM rooms;")
	if err != nil {
		err = errors.New("Postgres query error: " + err.Error())
		log.Println(err.Error())
		return "", err
	}
	defer rows.Close()

	results := make([][]string, 0)
	for rows.Next() {
		var room_id string
		var ip string
		var timestamp time.Time
		err = rows.Scan(&room_id, &ip, &timestamp)
		if err != nil {
			err = errors.New("Postgres row scan: " + err.Error())
			log.Println(err.Error())
			return "", err
		}
		results = append(results, []string{room_id, ip, timestamp.String()})
	}
	res, _ := json.Marshal(results)
	log.Println("Database:\n", string(res))

	return string(res), nil
}

func (d *Database) fetch_an_non_self_ip(self_ip string) (string, error) {
	conn, err := pgxw.Connect(psqlurl)
	if err != nil {
		err = errors.New("Postgres connection error: " + err.Error())
		log.Println(err.Error())
		return "", err
	}
	defer conn.Close()

	var target_ip string
	err = conn.QueryRow("SELECT task_private_ip FROM rooms WHERE task_private_ip!=$1 LIMIT 1;", self_ip).Scan(&target_ip)
	if err != nil {
		err = errors.New("Postgres query error: " + err.Error())
		log.Println(err.Error())
		return "", err
	}

	return target_ip, nil
}

// TODO: add a database init retry for 30 minutes (once every 30s)

func (d *Database) initialize() error {
	conn, err := pgxw.Connect(psqlurl)
	if err != nil {
		err = errors.New("Postgres connection error: " + err.Error())
		log.Println(err.Error())
		return err
	}
	defer conn.Close()

	// Check if rooms table exists; if not, populate the database
	var table_exist bool
	err = conn.QueryRow("SELECT EXISTS (SELECT FROM pg_tables WHERE schemaname='public' AND tablename='rooms');").Scan(&table_exist)
	if err != nil {
		err = errors.New("Postgres query error: " + err.Error())
		log.Println(err.Error())
		return err
	}
	if table_exist {
		log.Println("Schema was populated.")
		return nil
	} else {
		log.Println("Populating schema.")
	}

	path := filepath.Join("postgres", "create_tables.sql")

	c, err := ioutil.ReadFile(path)
	if err != nil {
		err = errors.New("Read create_tables.sql error: " + err.Error())
		log.Println(err.Error())
		return err
	}
	sql := string(c)

	_, err = conn.Exec(sql)
	if err != nil {
		err = errors.New("Postgres schema population error: " + err.Error())
		log.Println(err.Error())
		return err
	}
	return nil
}

var DB = Database{}
