package main

import (
	"net/http"

	"testing"

	"github.com/gavv/httpexpect/v2"
)

var TEST_TENDER_ID = "" // will be set by TestCreateTender

const TEST_ORG_ID = "550e8400-e29b-41d4-a716-446655440000"

func TestPing(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080")

	e.GET("/api/ping").
		Expect().
		Status(http.StatusOK).
		Body().IsEqual("ok")
}

func TestGetTenders(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080")

	e.GET("/api/tenders").
		Expect().
		Status(http.StatusOK).
		JSON().Array().NotEmpty()
}

func TestCreateTender(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080")

	response := e.POST("/api/tenders/new").
		WithJSON(map[string]interface{}{
			"name":            "Тендер 1",
			"description":     "Описание тендера",
			"serviceType":     "Construction",
			"organizationId":  TEST_ORG_ID,
			"creatorUsername": "test_user",
		}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	response.Value("createdAt").String().NotEmpty()
	response.Value("description").String().IsEqual("Описание тендера")
	response.Value("id").String().NotEmpty() // id должен быть не пустым UUID
	response.Value("name").String().IsEqual("Тендер 1")
	response.Value("organizationId").String().IsEqual(TEST_ORG_ID)
	response.Value("serviceType").String().IsEqual("Construction")
	response.Value("status").String().IsEqual("CREATED")
	response.Value("version").Number().IsEqual(1)
	TEST_TENDER_ID = response.Value("id").String().Raw()
}

func TestGetUserTenders(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080")

	e.GET("/api/tenders/my").
		WithQuery("username", "test_user").
		Expect().
		Status(http.StatusOK).
		JSON().Array().NotEmpty()
}

func TestGetTenderStatus(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080")

	e.GET("/api/tenders/"+TEST_TENDER_ID+"/status").
		WithQuery("username", "test_user").
		Expect().
		Status(http.StatusOK).
		Body().IsEqual("\"CREATED\"\n")
}

func TestPutTenderStatus(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080")

	response := e.PUT("/api/tenders/"+TEST_TENDER_ID+"/status").
		WithQuery("username", "test_user").
		WithQuery("status", "Published").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	response.Value("createdAt").String().NotEmpty()
	response.Value("description").String().IsEqual("Описание тендера")
	response.Value("id").String().NotEmpty()
	response.Value("name").String().IsEqual("Тендер 1")
	response.Value("organizationId").String().NotEmpty()
	response.Value("serviceType").String().IsEqual("Construction")
	response.Value("status").String().IsEqual("PUBLISHED")
}

func TestEditTender(t *testing.T) {
	e := httpexpect.Default(t, "http://localhost:8080")

	response := e.PATCH("/api/tenders/"+TEST_TENDER_ID+"/edit").
		WithQuery("username", "test_user").
		WithJSON(map[string]interface{}{
			"name":        "Обновленный Тендер 1",
			"description": "Обновленное описание",
			"serviceType": "Construction",
		}).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	response.Value("createdAt").String().NotEmpty()
	response.Value("description").String().IsEqual("Обновленное описание")
	response.Value("id").String().IsEqual(TEST_TENDER_ID)
	response.Value("name").String().IsEqual("Обновленный Тендер 1")
	response.Value("organizationId").String().NotEmpty()
	response.Value("serviceType").String().IsEqual("Construction")
	response.Value("status").String().IsEqual("PUBLISHED")
}
