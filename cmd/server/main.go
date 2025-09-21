package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Docx Editor by Placeholder Method - Prototype")
	fmt.Println("===========================================")

	inputFile := "../../example.docx"
	outputFile := "../../output_modified.docx"

	processor := NewDocxProcessor(inputFile, outputFile)

	if err := processor.Process(); err != nil {
		log.Fatalf("Error processing docx file: %v", err)
	}

	fmt.Println("\nDocx processing completed successfully!")
	fmt.Printf("Output file: %s\n", outputFile)
}
