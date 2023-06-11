package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
)

// GetDeviceMetadata lists metadata for a device
// @Summary      Get Device Metadata
// @Id  		 GetDeviceMetadata
// @Tags         Devices
// @Description  Lists metadata for a device
// @Param        id   path      string  true "Device ID"
// @Accept	     json
// @Produce      json
// @Success      200  {object}  models.DeviceMetadata
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata [get]
func (api *API) GetDeviceMetadata(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.NewApiInternalError(fmt.Errorf("not implemented")))
}

// GetDeviceMetadata Get value for a metadata key on a device
// @Summary      Get Device Metadata
// @Id  		 GetDeviceMetadataKey
// @Tags         Devices
// @Description  Get metadata for a device
// @Param        id   path      string  true "Device ID"
// @Param        key  path      string  true "Metadata Key"
// @Accept	     json
// @Produce      json
// @Success      200  {object}  models.DeviceMetadataValue
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata/{key} [get]
func (api *API) GetDeviceMetadataKey(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.NewApiInternalError(fmt.Errorf("not implemented")))
}

// UpdateDeviceMetadataKey Set value for a metadata key on a device
// @Summary      Set Device Metadata by key
// @Id  		 UpdateDeviceMetadataKey
// @Tags         Devices
// @Description  Set metadata key for a device
// @Param        id   path      string  true "Device ID"
// @Param        key  path      string  false "Metadata Key"
// @Param		 update body models.DeviceMetadataValue true "Metadata Value"
// @Accept	     json
// @Produce      json
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata/{key} [post]
func (api *API) UpdateDeviceMetadataKey(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.NewApiInternalError(fmt.Errorf("not implemented")))
}

// DeleteDeviceMetadata Delete all metadata or a specific key on a device
// @Summary      Delete all Device metadata
// @Id  		 DeleteDeviceMetadata
// @Tags         Devices
// @Description  Delete all metadata for a device
// @Param        id   path      string  true "Device ID"
// @Param        key  path      string  false "Metadata Key"
// @Accept	     json
// @Produce      json
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata [post]
func (api *API) DeleteDeviceMetadata(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.NewApiInternalError(fmt.Errorf("not implemented")))
}

// DeleteDeviceMetadataKey Delete all metadata or a specific key on a device
// @Summary      Delete a Device metadata key
// @Id  		 DeleteDeviceMetadataKey
// @Tags         Devices
// @Description  Delete a metadata key for a device
// @Param        id   path      string  true "Device ID"
// @Param        key  path      string  false "Metadata Key"
// @Accept	     json
// @Produce      json
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata/{key} [post]
func (api *API) DeleteDeviceMetadataKey(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.NewApiInternalError(fmt.Errorf("not implemented")))
}
