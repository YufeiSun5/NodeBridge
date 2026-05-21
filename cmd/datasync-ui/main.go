package main

import "fmt"

func main() {
	app := NewApp()
	overview := app.GetOverview()
	fmt.Printf("%s ui backend ready status=%s\n", overview.ProductName, overview.AgentStatus)
}
