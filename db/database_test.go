package db

import (
	"fmt"
	"testing"
)

func TestSqliteDB(t *testing.T) {
	t.SkipNow()
	//if err := os.Remove("./chatops.db"); err != nil {
	//	t.Fatal(err)
	//}
	file := fmt.Sprintf("file:chatops.db?_auth&_auth_user=%s&_auth_pass=%s&_see_key=asdf1312312", "test1", "test1")
	db := NewSqliteDB(file)
	if err := db.Init(); err != nil {
		t.Fatal(err)
	}

	if list, err := db.GetAllSlackBots(); err != nil {
		t.Fatal(err)
	} else {
		for _, b := range list {
			fmt.Println(b)
		}
	}

	if err := db.InsertSlackBot("test123", "test323", "999"); err != nil {
		t.Fatal(err)
	}
	//
	//if list, err := db.GetAllSlackBots(); err != nil {
	//	t.Fatal(err)
	//} else {
	//	for _, b := range list {
	//		fmt.Println(b)
	//	}
	//}
}
