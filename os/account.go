package os

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type User struct {
	UID      int
	GID      int
	Name     string
	Password string
	Homedir  string
	Info     string
	Shell    string
}

type Group struct {
	GID      int
	Name     string
	Userlist []string
}

var (
	users           = make(map[int]User)
	usernameMapping = make(map[string]User)
	groups          = make(map[int]Group)
)

func LoadUsers(userFile string) error {
	f, err := os.OpenFile(userFile, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ":")
		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			return err
		}
		gid, err := strconv.Atoi(fields[3])
		if err != nil {
			return err
		}
		userObj := User{
			UID:      uid,
			GID:      gid,
			Name:     fields[0],
			Password: fields[1],
			Info:     fields[4],
			Homedir:  fields[5],
			Shell:    fields[6],
		}
		users[uid] = userObj
		usernameMapping[fields[0]] = userObj
	}

	return nil
}

func LoadGroups(groupFile string) error {
	f, err := os.OpenFile(groupFile, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ":")
		gid, err := strconv.Atoi(fields[2])
		if err != nil {
			return err
		}
		groups[gid] = Group{
			GID:  gid,
			Name: fields[0],
		}
	}

	return nil
}

func IsUserExist(user string) (pass string, exists bool) {
	userObj, exists := usernameMapping[user]
	if !exists {
		return
	}
	return userObj.Password, exists
}

func GetUser(name string) User {
	return usernameMapping[name]
}

func GetUserByID(id int) User {
	return users[id]
}

func GetGroupByID(id int) Group {
	return groups[id]
}
