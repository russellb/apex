// Package docs_private Code generated by swaggo/swag. DO NOT EDIT
package docs_private

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {
            "name": "The Nexodus Authors",
            "url": "https://github.com/nexodus-io/nexodus/issues"
        },
        "license": {
            "name": "Apache 2.0",
            "url": "http://www.apache.org/licenses/LICENSE-2.0.html"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/admin/gc": {
            "post": {
                "description": "Cleans up old soft deleted records",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Private"
                ],
                "summary": "Cleans up old soft deleted records",
                "operationId": "GarbageCollect",
                "parameters": [
                    {
                        "type": "string",
                        "description": "how long to retain deleted records.  defaults to '24h'",
                        "name": "retention",
                        "in": "query"
                    }
                ],
                "responses": {
                    "204": {
                        "description": "No Content"
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/models.ValidationError"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/models.InternalServerError"
                        }
                    }
                }
            }
        },
        "/private/live": {
            "post": {
                "description": "Checks if the service is live",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Private"
                ],
                "summary": "Checks if the service is live",
                "operationId": "Live",
                "responses": {
                    "200": {
                        "description": "OK"
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/models.InternalServerError"
                        }
                    }
                }
            }
        },
        "/private/ready": {
            "post": {
                "description": "Checks if the service is ready to accept requests",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Private"
                ],
                "summary": "Checks if the service is ready to accept requests",
                "operationId": "Ready",
                "responses": {
                    "200": {
                        "description": "OK"
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/models.InternalServerError"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "models.InternalServerError": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "string",
                    "example": "something bad"
                },
                "trace_id": {
                    "type": "string"
                }
            }
        },
        "models.ValidationError": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "string",
                    "example": "something bad"
                },
                "field": {
                    "type": "string"
                },
                "reason": {
                    "type": "string"
                }
            }
        }
    },
    "securityDefinitions": {
        "OAuth2Implicit": {
            "type": "oauth2",
            "flow": "implicit",
            "authorizationUrl": "https://auth.try.nexodus.127.0.0.1.nip.io/",
            "scopes": {
                "admin": " Grants read and write access to administrative information",
                "user": " Grants read and write access to resources owned by this user"
            }
        }
    },
    "tags": [
        {
            "description": "X509 Certificate related APIs, these APIs are experimental and disabled by default.  Use the feature flag apis to check if they are enabled on the server.",
            "name": "CA"
        },
        {
            "description": "Skupper Site related APIs, these APIs are experimental and disabled by default.  Use the feature flag apis to check if they are enabled on the server.",
            "name": "Sites"
        }
    ]
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "",
	BasePath:         "/",
	Schemes:          []string{},
	Title:            "Nexodus API",
	Description:      "This is the Nexodus API Server.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
