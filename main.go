package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/gin-gonic/gin"
)

func main() {
	// Configuração do Gin
	router := gin.Default()

	// Rota para geração de texto com streaming
	router.POST("/generate-text", func(c *gin.Context) {
		// Extrair o prompt do corpo da solicitação
		var requestBody struct {
			Prompt string `json:"prompt"`
		}
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Validar se o prompt foi fornecido e não está vazio
		if requestBody.Prompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt cannot be empty"})
			return
		}

		// Configuração do cliente AWS
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1")) // Defina a região correta
		if err != nil {
			log.Fatalf("unable to load SDK config, %v", err)
		}

		// Criação do cliente Bedrock Runtime
		client := bedrockruntime.NewFromConfig(cfg)

		// Criação da solicitação de geração de texto
		input := &bedrockruntime.InvokeModelWithResponseStreamInput{
			ModelId:     aws.String("amazon.titan-text-lite-v1"), // Nome correto do modelo
			ContentType: aws.String("application/json"),
			Body: []byte(`{
				"inputText": "` + requestBody.Prompt + `",
				"textGenerationConfig": {
					"maxTokenCount": 512,
					"stopSequences": [],
					"temperature": 0.7,
					"topP": 1.0
				}
			}`),
		}

		// Invocação do modelo com streaming de resposta
		output, err := client.InvokeModelWithResponseStream(context.TODO(), input)
		if err != nil {
			log.Printf("failed to invoke model: %v", err) // Log do erro
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to invoke model"})
			return
		}

		// Configuração do streaming de resposta HTTP
		c.Stream(func(w io.Writer) bool {
			for event := range output.GetStream().Events() {
				switch v := event.(type) {
				case *types.ResponseStreamMemberChunk:
					// Acessar o campo Bytes do PayloadPart
					chunkBytes := v.Value.Bytes
					// Enviar o chunk de texto para o cliente
					c.SSEvent("message", string(chunkBytes))
				case *types.UnknownUnionMember:
					// Tratamento de eventos desconhecidos
					c.SSEvent("error", "unknown event")
				default:
					// Tratamento de outros tipos de eventos
					c.SSEvent("error", "unexpected event type")
				}
			}
			return false // Encerra o streaming após todos os eventos
		})
	})

	// Iniciar o servidor
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("failed to start server, %v", err)
	}
}
