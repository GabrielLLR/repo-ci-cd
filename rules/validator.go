package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/pb33f/libopenapi/index"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"gopkg.in/yaml.v3"
)

// Função para converter para UTF-8
func convertToUTF8(data []byte) ([]byte, error) {
	utf8Bom := unicode.BOMOverride(transform.Nop) // Remove BOM se existir
	reader := transform.NewReader(bytes.NewReader(data), utf8Bom)
	convertedData, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("erro ao converter encoding para UTF-8: %v", err)
	}
	return convertedData, nil
}

// Função para ler um arquivo local, converter para UTF-8 e retornar os bytes
func readFile(filePath string) ([]byte, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o arquivo %s: %v", filePath, err)
	}

	// Converte para UTF-8 antes de processar
	utf8Data, err := convertToUTF8(data)
	if err != nil {
		return nil, err
	}

	return utf8Data, nil
}

// Função para carregar as regras personalizadas do pb33f_rules.yaml
func loadRules(ruleFile string) (map[string]interface{}, error) {
	data, err := ioutil.ReadFile(ruleFile)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler regras YAML: %v", err)
	}

	var rules map[string]interface{}
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("erro ao fazer unmarshal das regras: %v", err)
	}

	return rules, nil
}

// Função para validar um arquivo OpenAPI usando regras personalizadas
func validateOpenAPIWithRules(filePath string, rulesFile string) error {
	// Ler o arquivo OpenAPI e converter para UTF-8
	data, err := readFile(filePath)
	if err != nil {
		return err
	}

	// Criar um nó YAML a partir do arquivo
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return fmt.Errorf("erro ao fazer unmarshal do YAML: %v", err)
	}

	// Criar configuração de indexação
	indexConfig := index.CreateClosedAPIIndexConfig()

	// Criar um indexador para a especificação OpenAPI
	idx := index.NewSpecIndexWithConfig(&rootNode, indexConfig)
	// Obter erros básicos do OpenAPI
	validationErrors := idx.GetReferenceIndexErrors()

	// Carregar regras personalizadas
	rules, err := loadRules(rulesFile)
	if err != nil {
		return err
	}

	// Aplicar regras personalizadas
	if rulesMap, ok := rules["rules"].(map[string]interface{}); ok {
		for ruleName, rule := range rulesMap {
			ruleData := rule.(map[string]interface{})
			// given := ruleData["given"].(string)
			fmt.Println(ruleName)
			severity := ruleData["severity"].(string)

			// Aplicação manual de regras
			if ruleName == "enforce-security" {
				if idx.GetAllSecuritySchemes() == nil {
					validationErrors = append(validationErrors, fmt.Errorf("[%s] %s", severity, ruleData["description"]))
				}
			}

			if ruleName == "openapi-tags" {
				if idx.GetTotalTagsCount() == 0 {
					validationErrors = append(validationErrors, fmt.Errorf("[%s] %s", severity, ruleData["description"]))
				}
			}

			if ruleName == "require-contact-info" {
				validationErrors = validarInfo(rootNode.Content[0], &validationErrors, severity, ruleData, "contact")
			}

			if ruleName == "info-title" {
				validationErrors = validarInfo(rootNode.Content[0], &validationErrors, severity, ruleData, "title")
			}

			if ruleName == "info-description" {
				validationErrors = validarInfo(rootNode.Content[0], &validationErrors, severity, ruleData, "description")
			}

			if ruleName == "info-version" {
				var infoVal string

				aquivoNode := rootNode.Content[0]
				for i := 0; i < len(aquivoNode.Content); i += 2 {
					nodeArquivo := aquivoNode.Content[i]
					if nodeArquivo.Kind == yaml.ScalarNode && nodeArquivo.Value == "info" {
						infoNode := aquivoNode.Content[i+1]
						for j := 0; j < len(infoNode.Content); j += 2 {
							nodeInfo := infoNode.Content[j]
							if nodeInfo.Kind == yaml.ScalarNode && nodeInfo.Value == "version" {
								infoVal = infoNode.Content[j+1].Value
							}
						}
					}
				}

				re := regexp.MustCompile(`^(\d+\.\d+\.\d+)(?:-(rc|beta)\.\d+)?$`)
				if !re.MatchString(strings.TrimSpace(string(infoVal))) {
					validationErrors = append(validationErrors, fmt.Errorf("[%s] %s", severity, ruleData["description"]))
				}
			}

			if ruleName == "only-https" {
				servers := idx.GetAllRootServers()
				for i := 0; i < len(servers); i += 1 {
					infoVal, _ := yaml.Marshal(servers[i].Node.Content[1])
					url := strings.Trim(strings.TrimSpace(string(infoVal)), "'")

					if !strings.HasPrefix(url, "https://") {
						validationErrors = append(validationErrors, fmt.Errorf("[%s] %s", severity, ruleData["description"]))
					}
				}
			}

			if ruleName == "paths-kebab-case" {
				for path := range idx.GetAllPaths() {

					re := regexp.MustCompile(`\{[^}]+\}`)
					pathNoVariable := re.ReplaceAllString(path, "")
					kebabRegex := regexp.MustCompile(`^\/([a-z0-9]+(-[a-z0-9]+)*)+(\/[a-z0-9]+(-[a-z0-9]+)*)*\/?$`)

					if !kebabRegex.MatchString(pathNoVariable) {
						validationErrors = append(validationErrors, fmt.Errorf("[%s] %s", severity, ruleData["description"]))
					}

				}
			}

			if ruleName == "operation-operationId" {
				validationErrors = validarPaths(idx.GetAllPaths(), &validationErrors, severity, ruleData, "operationId")
			}

			if ruleName == "operation-tags" {
				validationErrors = validarPaths(idx.GetAllPaths(), &validationErrors, severity, ruleData, "tags")
			}

			if ruleName == "pattern-found-NA" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarPattern(schemaValidado, &validationErrors, severity, ruleData, `\bNA\b`, schema)
				}
			}

			if ruleName == "pattern-found-texto" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarPattern(schemaValidado, &validationErrors, severity, ruleData, `\\w*\\W*`, schema)
				}
			}

			if ruleName == "transaction-found-last" {
				allComponents := idx.GetAllComponentSchemas()
				for schema := range allComponents {
					schemas := strings.Split(schema, "/")
					if schemas[len(schemas)-1] == "TransactionsLinks" {
						transactionsLinks := allComponents[schema].Node.Content
						for i := 0; i < len(transactionsLinks); i += 2 {
							if transactionsLinks[i].Value == "properties" {
								properties := transactionsLinks[i+1].Content
								for j := 0; j < len(properties); j += 2 {
									if properties[j].Value == "last" {
										validationErrors = append(validationErrors, fmt.Errorf("[%s] %s", severity, ruleData["description"]))
									}
								}
							}
						}
					}
				}
			}

			//biblioteca ja retorna o valor com trim, então não consegue fazer esta validação
			if ruleName == "no-leading-trailing-spaces" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarPatternString(schemaValidado, &validationErrors, severity, ruleData, schema)
				}
			}

			//também valida objects-required-in-request-should-has-properties-response
			if ruleName == "objects-required-in-request-should-has-properties-request" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarObjeto(schemaValidado, &validationErrors, severity, ruleData, schema)
				}
			}

			if ruleName == "string-should-has-maxLength" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarPropriedadeString(schemaValidado, &validationErrors, severity, ruleData, "maxLength", schema)
				}
			}

			if ruleName == "string-should-has-minLength" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarPropriedadeString(schemaValidado, &validationErrors, severity, ruleData, "minLength", schema)
				}
			}

			if ruleName == "string-should-has-pattern" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarPropriedadeString(schemaValidado, &validationErrors, severity, ruleData, "pattern", schema)
				}
			}

			if ruleName == "no-maxLentgh-for-enum" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarPropriedadeEnum(schemaValidado, &validationErrors, severity, ruleData, "maxLength", schema)
				}
			}

			if ruleName == "no-minLength-for-enum" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarPropriedadeEnum(schemaValidado, &validationErrors, severity, ruleData, "minLength", schema)
				}
			}

			if ruleName == "array-objects-max-items" {
				for schema := range idx.GetAllComponentSchemas() {
					schemaValidado := idx.GetAllComponentSchemas()[schema].Node.Content
					validationErrors = validarArrayMaxItems(schemaValidado, &validationErrors, severity, ruleData, schema)
				}
			}
		}
	}

	// Exibir erros encontrados
	if len(validationErrors) > 0 {
		var onlyWarn = true
		for _, err := range validationErrors {
			re := regexp.MustCompile(`\berror\b`)

			if re.MatchString(err.Error()) {
				onlyWarn = false
			}
			
			fmt.Println("❌ Erro de validação:", err)
		}
		if !onlyWarn {
			return fmt.Errorf("falha na validação do OpenAPI")
		}
	}

	fmt.Println("✅ OpenAPI válido com regras aplicadas:", filePath)
	return nil
}

func validarPaths(paths map[string]map[string]*index.Reference, validationErrors *[]error, severity string, ruleData map[string]interface{}, campo string) []error {
	for _, methods := range paths {
		for _, ref := range methods {
			request := ref.Node.Content
			for i := 0; i < len(request); i += 2 {
				if request[i].Kind == yaml.ScalarNode && request[i].Value == campo {

					valorCampo, _ := yaml.Marshal(request[i+1])

					if len(strings.TrimSpace(string(valorCampo))) == 0 {
						*validationErrors = append(*validationErrors, fmt.Errorf("[%s] %s", severity, ruleData["description"]))
					}
				}
			}
		}
	}
	return *validationErrors
}

func validarInfo(aquivoNode *yaml.Node, validationErrors *[]error, severity string, ruleData map[string]interface{}, campo string) []error {
	var infoVal string

	for i := 0; i < len(aquivoNode.Content); i += 2 {
		nodeArquivo := aquivoNode.Content[i]
		if nodeArquivo.Kind == yaml.ScalarNode && nodeArquivo.Value == "info" {
			infoNode := aquivoNode.Content[i+1]
			for j := 0; j < len(infoNode.Content); j += 2 {
				nodeInfo := infoNode.Content[j]
				if nodeInfo.Kind == yaml.ScalarNode && nodeInfo.Value == campo {
					infoVal = infoNode.Content[j+1].Value
				}
			}
		}
	}

	if strings.TrimSpace(infoVal) == "" {
		*validationErrors = append(*validationErrors, fmt.Errorf("[%s] %s", severity, ruleData["description"]))
	}

	return *validationErrors
}

func validarArrayMaxItems(schema []*yaml.Node, validationErrors *[]error, severity string, ruleData map[string]interface{}, campo string) []error {
	for i := 0; i < len(schema); i += 2 {
		if schema[i+1].Value == "array" {
			var hasMaxItems = false
			for j := 0; j < len(schema); j += 2 {
				if schema[j].Value == "maxItems" {
					hasMaxItems = true
					break
				}
			}
			if !hasMaxItems {
				*validationErrors = append(*validationErrors, fmt.Errorf("[%s] campo: %s - %s", severity, campo, ruleData["description"]))
			}
		}
		if schema[i].Value == "properties" {
			subSchema := schema[i+1].Content
			for j := 0; j < len(subSchema); j += 2 {
				*validationErrors = validarArrayMaxItems(subSchema[j+1].Content, validationErrors, severity, ruleData, campo+"/"+subSchema[j].Value)
			}
		}
		if schema[i].Value == "items" {
			*validationErrors = validarArrayMaxItems(schema[i+1].Content, validationErrors, severity, ruleData, campo+"/"+schema[i+1].Content[0].Value)
		}
	}
	return *validationErrors
}

func validarPropriedadeString(schema []*yaml.Node, validationErrors *[]error, severity string, ruleData map[string]interface{}, propriedade string, campo string) []error {
	for i := 0; i < len(schema); i += 2 {
		if schema[i+1].Value == "string" {
			var hasPropriedade = false
			var isEnum = false
			for j := 0; j < len(schema); j += 2 {
				if schema[j].Value == "enum" {
					isEnum = true
					break
				} else {
					if schema[j].Value == propriedade {
						hasPropriedade = true
						break
					}
				}
			}
			if !hasPropriedade && !isEnum {
				*validationErrors = append(*validationErrors, fmt.Errorf("[%s] campo: %s - %s", severity, campo, ruleData["description"]))
			}
		}
		if schema[i].Value == "properties" {
			subSchema := schema[i+1].Content
			for j := 0; j < len(subSchema); j += 2 {
				*validationErrors = validarPropriedadeString(subSchema[j+1].Content, validationErrors, severity, ruleData, propriedade, campo+"/"+subSchema[j].Value)
			}
		}
		if schema[i].Value == "items" {
			*validationErrors = validarPropriedadeString(schema[i+1].Content, validationErrors, severity, ruleData, propriedade, campo+"/"+schema[i+1].Content[0].Value)
		}
	}
	return *validationErrors
}

func validarPropriedadeEnum(schema []*yaml.Node, validationErrors *[]error, severity string, ruleData map[string]interface{}, propriedade string, campo string) []error {
	for i := 0; i < len(schema); i += 2 {
		if schema[i].Value == "enum" {
			var hasPropriedade = false
			for j := 0; j < len(schema); j += 2 {
				if schema[j].Value == propriedade {
					hasPropriedade = true
					break
				}
			}
			if hasPropriedade {
				*validationErrors = append(*validationErrors, fmt.Errorf("[%s] campo: %s - %s", severity, campo, ruleData["description"]))
			}
		}
		if schema[i].Value == "properties" {
			subSchema := schema[i+1].Content
			for j := 0; j < len(subSchema); j += 2 {
				*validationErrors = validarPropriedadeEnum(subSchema[j+1].Content, validationErrors, severity, ruleData, propriedade, campo+"/"+subSchema[j].Value)
			}
		}
		if schema[i].Value == "items" {
			*validationErrors = validarPropriedadeEnum(schema[i+1].Content, validationErrors, severity, ruleData, propriedade, campo+"/"+schema[i+1].Content[0].Value)
		}
	}
	return *validationErrors
}


func validarPattern(schema []*yaml.Node, validationErrors *[]error, severity string, ruleData map[string]interface{}, pattern string, campo string) []error {
	for i := 0; i < len(schema); i += 2 {
		if schema[i].Value == "pattern" {
			re := regexp.MustCompile(pattern)

			if re.MatchString(schema[i+1].Value) {
				*validationErrors = append(*validationErrors, fmt.Errorf("[%s] campo: %s - %s", severity, campo, ruleData["description"]))
			}
		}
		if schema[i].Value == "properties" {
			subSchema := schema[i+1].Content
			for j := 0; j < len(subSchema); j += 2 {
				*validationErrors = validarPattern(subSchema[j+1].Content, validationErrors, severity, ruleData, pattern, campo+"/"+subSchema[j].Value)
			}
		}
		if schema[i].Value == "items" {
			*validationErrors = validarPattern(schema[i+1].Content, validationErrors, severity, ruleData, pattern, campo+"/"+schema[i+1].Content[0].Value)
		}
	}
	return *validationErrors
}

func validarPatternString(schema []*yaml.Node, validationErrors *[]error, severity string, ruleData map[string]interface{}, campo string) []error {
	var example = ""
	for i := 0; i < len(schema); i += 2 {
		if schema[i].Value == "properties" {
			subSchema := schema[i+1].Content
			for j := 0; j < len(subSchema); j += 2 {
				*validationErrors = validarPatternString(subSchema[j+1].Content, validationErrors, severity, ruleData, campo+"/"+subSchema[j].Value)
			}
		}
		if schema[i].Value == "items" {
			*validationErrors = validarPatternString(schema[i+1].Content, validationErrors, severity, ruleData, campo+"/"+schema[i+1].Content[0].Value)
		}
		if example == "" {
			if schema[i].Value == "enum" {
				break
			}
			if schema[i+1].Value == "string" {
				for j := 0; j < len(schema); j += 2 {
					if schema[j].Value == "pattern" {
						example = schema[j+1].Value
					}
				}
			}
		}
	}
	if example != "" && strings.TrimSpace(example) != example {
		*validationErrors = append(*validationErrors, fmt.Errorf("[%s] campo: %s - %s", severity, campo, ruleData["description"]))
	}
	return *validationErrors
}

func validarObjeto(schema []*yaml.Node, validationErrors *[]error, severity string, ruleData map[string]interface{}, campo string) []error {
	for i := 0; i < len(schema); i += 2 {
		if schema[i].Value == "properties" {
			subSchema := schema[i+1].Content
			for j := 0; j < len(subSchema); j += 2 {
				*validationErrors = validarObjeto(subSchema[j+1].Content, validationErrors, severity, ruleData, campo+"/"+subSchema[j].Value)
			}
		}
		if schema[i].Value == "items" {
			*validationErrors = validarObjeto(schema[i+1].Content, validationErrors, severity, ruleData, campo+"/"+schema[i+1].Content[0].Value)
		}
		if schema[i+1].Value == "object" {
			var requiredFields []string
			var properiesFields []string
			for j := 0; j < len(schema); j += 2 {
				// campo+"/"+schema[j+1].Content[0].Value
				if schema[j].Value == "required" {
					required := schema[j+1].Content
					for k := 0; k < len(required); k++ {

						requiredFields = append(requiredFields, required[k].Value)
					}
				}
				if schema[j].Value == "properties" {
					properties := schema[j+1].Content
					for k := 0; k < len(properties); k += 2 {

						properiesFields = append(properiesFields, properties[k].Value)
					}
				}
			}
			set := make(map[string]bool)
			for _, item := range properiesFields {
				set[item] = true
			}
			for _, item := range requiredFields {
				if !set[item] {
					*validationErrors = append(*validationErrors, fmt.Errorf("[%s] campo: %s - %s", severity, campo+"/"+item, ruleData["description"]))
				}
			}
		}
	}
	return *validationErrors
}

// Função para resolver referências OpenAPI e salvar o arquivo resolvido
func resolveOpenAPI(inputFile, outputFile string) error {
	// Ler o arquivo e converter para UTF-8
	data, err := readFile(inputFile)
	if err != nil {
		return err
	}

	// Criar um nó YAML a partir do arquivo
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return fmt.Errorf("erro ao fazer unmarshal do YAML: %v", err)
	}

	// Criar uma configuração para o indexador
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
	resolvedYAML, err := yaml.Marshal(&rootNode)
	if err != nil {
		return fmt.Errorf("erro ao converter para YAML: %v", err)
	}

	// Salvar o YAML resolvido em um novo arquivo
	if err := ioutil.WriteFile(outputFile, resolvedYAML, 0644); err != nil {
		return fmt.Errorf("erro ao salvar arquivo resolvido: %v", err)
	}

	fmt.Println("Arquivo resolvido salvo em:", outputFile)
	return nil
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Uso: go run main.go oldSwagger.yaml swagger.yaml pb33f_rules.yaml")
		return
	}

	oldFile := os.Args[1]
	newFile := os.Args[2]
	rulesFile := os.Args[3]

	// Validar arquivos com regras personalizadas antes de resolver
	if err := validateOpenAPIWithRules(oldFile, rulesFile); err != nil {
		fmt.Println("❌ OpenAPI inválido:", oldFile)
		os.Exit(1)
	}

	if err := validateOpenAPIWithRules(newFile, rulesFile); err != nil {
		fmt.Println("❌ OpenAPI inválido:", newFile)
		os.Exit(1)
	}

	// Resolver e salvar os arquivos
	if err := resolveOpenAPI(oldFile, "oldSwaggerResolve.yaml"); err != nil {
		fmt.Println("❌ Erro ao processar oldSwagger.yaml:", err)
		os.Exit(1)
	}

	if err := resolveOpenAPI(newFile, "swaggerResolve.yaml"); err != nil {
		fmt.Println("❌ Erro ao processar swagger.yaml:", err)
		os.Exit(1)
	}

	fmt.Println("🚀 OpenAPI validado com regras aplicadas e arquivos resolvidos gerados com sucesso!")
}
