package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pb33f/libopenapi/index"
	"gopkg.in/yaml.v3"
)

// Função para ler um arquivo local e retornar seu conteúdo como bytes
func readFile(filePath string) ([]byte, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o arquivo %s: %v", filePath, err)
	}
	return data, nil
}

// Função para resolver as referências OpenAPI usando o rolodex
func resolveOpenAPI(inputFile, outputFile string) error {
	// Ler o arquivo de entrada
	data, err := readFile(inputFile)
	if err != nil {
		return err
	}

	// Criar um nó YAML a partir do arquivo
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return fmt.Errorf("erro ao fazer unmarshal do YAML: %v", err)
	}

	// Criar uma configuração para o indexador (desabilitando lookups externos)
	indexConfig := index.CreateClosedAPIIndexConfig()

	// Criar um novo rolodex para gerenciar referências
	rolodex := index.NewRolodex(indexConfig)

	// Definir o root node do rolodex
	rolodex.SetRootNode(&rootNode)

	// Indexar as referências do OpenAPI
	if err := rolodex.IndexTheRolodex(); err != nil {
		return fmt.Errorf("erro ao indexar as referências: %v", err)
	}

	// Resolver todas as referências
	rolodex.Resolve()

	// Criar um YAML resolvido a partir do rolodex atualizado
	resolvedYAML, err := yaml.Marshal(rootNode)
	if err != nil {
		return fmt.Errorf("erro ao converter para YAML: %v", err)
	}

	// Salvar o YAML resolvido em um novo arquivo
	if err := ioutil.WriteFile(outputFile, resolvedYAML, 0644); err != nil {
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
	if err := resolveOpenAPI(oldFile, "oldSwaggerResolve.yaml"); err != nil {
		fmt.Println("❌ Erro ao processar oldSwagger.yaml:", err)
		os.Exit(1)
	}

	if err := resolveOpenAPI(newFile, "swaggerResolve.yaml"); err != nil {
		fmt.Println("❌ Erro ao processar swagger.yaml:", err)
		os.Exit(1)
	}

	fmt.Println("🚀 OpenAPI validado e arquivos resolvidos gerados com sucesso!")
}
