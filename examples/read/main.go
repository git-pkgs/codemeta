// Command read demonstrates reading metadata from a file or standard input.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/git-pkgs/codemeta"
)

func main() {
	const argumentCount = 2
	const usageExitCode = 2
	if len(os.Args) != argumentCount {
		fmt.Fprintln(os.Stderr, "usage: read <codemeta.json|->")
		os.Exit(usageExitCode)
	}
	var doc *codemeta.Document
	var err error
	if os.Args[1] == "-" {
		doc, err = codemeta.Read(os.Stdin, codemeta.ParseOptions{})
	} else {
		doc, err = codemeta.ReadFile(os.Args[1], codemeta.ParseOptions{})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Name: %s\nContext: %s\nSoftware version: %s\n", doc.Name(), doc.Version(), strings.Join(doc.Strings("version"), ", "))
	printAuthors(doc.Author())
	for _, d := range doc.Validate() {
		fmt.Printf("%d:%d %s %s: %s\n", d.Line, d.Column, d.Path, d.Code, d.Message)
	}
}

func printAuthors(authors []codemeta.Agent) {
	for _, author := range authors {
		fmt.Printf("Author: %s %s %s\n", author.Name(), strings.Join(author.Strings("givenName"), " "), strings.Join(author.Strings("familyName"), " "))
		if author.Kind() == codemeta.AgentRole {
			fmt.Printf("Role: %s\n", strings.Join(author.Strings("roleName"), ", "))
			printAuthors(author.Agents())
		}
	}
}
