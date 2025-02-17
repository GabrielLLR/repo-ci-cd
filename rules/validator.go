package main

import (
	"fmt"
	"os"
	"github.com/pb33f/libopenapi"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run validate.go swagger.yaml")
		return
	}

	file := os.Args[1]
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Println("Erro ao ler o arquivo:", err)
		os.Exit(1)
	}

	doc, err := libopenapi.NewDocument(data)

	info := doc.GetSpecInfo()
	
	if info != nil {
		fmt.Println("📄 Versão do OpenAPI:", info.Version)
	} else {
		fmt.Println("❌ Não foi possível obter a versão do OpenAPI.")
	}

	if err != nil {
		fmt.Println("Erro ao carregar OpenAPI:", err)
		os.Exit(1)
	}

	fmt.Println("✅ OpenAPI validado com sucesso!")
}
