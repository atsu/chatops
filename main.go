package main

import "github.com/atsu/chatops/app"

const ApplicationName = "chatops"

func main() {
	app.NewChatOps(ApplicationName).Run()
}
