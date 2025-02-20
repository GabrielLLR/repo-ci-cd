package main

import (
	"fmt"
	"os"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/tools/resolve"
	"gopkg.in/yaml.v2"
)

// Função para resolver e salvar um arquivo OpenAPI resolvido
func resolveAndSave(inputFile, outputFile string) error {
	// Ler o arquivo original
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("erro ao ler o arquivo %s: %v", inputFile, err)
	}

	// Criar um novo documento OpenAPI
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return fmt.Errorf("erro ao processar OpenAPI: %v", err)
	}

	// Resolver todas as referências
	resolver := resolve.NewResolver(doc)
	resolvedDoc, _, err := resolver.Resolve()
	if err != nil {
		return fmt.Errorf("erro ao resolver referências: %v", err)
	}

	// Converter o OpenAPI resolvido para YAML
	resolvedYAML, err := yaml.Marshal(resolvedDoc)
	if err != nil {
		return fmt.Errorf("erro ao converter para YAML: %v", err)
	}

	// Salvar o arquivo resolvido
	err = os.WriteFile(outputFile, resolvedYAML, 0644)
	if err != nil {
		return fmt.Errorf("erro ao salvar arquivo resolvido: %v", err)
	}

	fmt.Println("✅ Arquivo resolvido salvo em:", outputFile)
	return nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Uso: go run main.go oldSwagger.yaml swagger.yaml")
		return
	}

	oldFile := os.Args[1]
	newFile := os.Args[2]

	// Resolver e salvar os arquivos
	if err := resolveAndSave(oldFile, "oldSwaggerResolve.yaml"); err != nil {
		fmt.Println("❌ Erro ao processar oldSwagger.yaml:", err)
		os.Exit(1)
	}

	if err := resolveAndSave(newFile, "swaggerResolve.yaml"); err != nil {
		fmt.Println("❌ Erro ao processar swagger.yaml:", err)
		os.Exit(1)
	}

	fmt.Println("🚀 OpenAPI validado e arquivos resolvidos gerados com sucesso!")
}
