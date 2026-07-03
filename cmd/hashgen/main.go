package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

func main() {
	flag.Parse()

	password := flag.Arg(0)
	if password == "" {
		password = readPassword()
	}

	if password == "" {
		log.Fatal("no password provided")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		log.Fatalf("could not generate hash: %v", err)
	}

	fmt.Println(string(hash))
}

func readPassword() string {
	fmt.Fprint(os.Stderr, "Password: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line)
}
