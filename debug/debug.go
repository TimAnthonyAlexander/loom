package debug

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func DumpAndDie(v any) {
	tea.ClearScreen() // exit alt screen to show output in terminal

	fmt.Println("====== DEBUG DUMP ======")
	fmt.Printf("%#v\n", v)
	fmt.Println("========================")

	os.Exit(1)
}

func LogToFile(v any) {
	f, err := os.OpenFile("/Users/tim.alexander/loom/debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	logger := log.New(f, "[DEBUG] ", log.LstdFlags|log.Lshortfile)
	logger.Printf("%#v", v)
}
