package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
)

func (suite *HandlerTestSuite) TestListOrganizations() {
	assert := suite.Assert()
	require := suite.Require()
	organizations := []models.AddOrganization{
		{
			Name:     "organization-a",
			IpCidr:   "10.1.1.0/24",
			IpCidrV6: "fc00::/20",
		},
		{
			Name:     "organization-b",
			IpCidr:   "10.1.2.0/24",
			IpCidrV6: "fc00:1000::/20",
		},
		{
			Name:     "organization-c",
			IpCidr:   "10.1.3.0/24",
			IpCidrV6: "fc00:2000::/20",
		},
	}
	organizationDenied := models.AddOrganization{
		Name:     "organization-denied-multi-organization-off",
		IpCidr:   "10.1.3.0/24",
		IpCidrV6: "fc00:3000::/20",
	}

	for _, organization := range organizations {
		reqBody, err := json.Marshal(organization)
		assert.NoError(err)
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			suite.api.CreateOrganization,
			bytes.NewBuffer(reqBody),
		)
		require.NoError(err)
		body, err := io.ReadAll(res.Body)
		require.NoError(err)
		require.Equal(http.StatusCreated, res.Code, string(body))

		var o models.OrganizationJSON
		err = json.Unmarshal(body, &o)
		require.NoError(err)
	}

	{
		resBody, err := json.Marshal(organizationDenied)
		assert.NoError(err)
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			func(c *gin.Context) {
				c.Set("nexodus.testCreateOrganization", "false")
				suite.api.CreateOrganization(c)
			},

			bytes.NewBuffer(resBody),
		)
		assert.NoError(err)
		assert.Equal(http.StatusMethodNotAllowed, res.Code)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", "/",
			suite.api.ListOrganizations, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.OrganizationJSON
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)
		assert.Len(actual, 4)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?sort=["name","DESC"]`,
			suite.api.ListOrganizations, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.OrganizationJSON
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)

		assert.Len(actual, 4)
		seen := map[string]bool{
			"testuser":       false,
			"organization-a": false,
			"organization-b": false,
			"organization-c": false,
		}
		for _, org := range actual {
			if _, ok := seen[org.Name]; ok {
				seen[org.Name] = true
			}
		}
		for k, v := range seen {
			assert.Equal(v, true, "organization %s was not seen", k)
		}
	}
	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?filter={"name":"default"}`,
			suite.api.ListOrganizations, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.Organization
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)

		assert.Len(actual, 0)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?range=[3,4]`,
			suite.api.ListOrganizations, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.OrganizationJSON
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)
		// The orgs are sorted by name..
		assert.Len(actual, 1)
		assert.Equal("4", res.Header().Get(TotalCountHeader))
		assert.Equal("testuser", actual[0].Name)
	}

}
