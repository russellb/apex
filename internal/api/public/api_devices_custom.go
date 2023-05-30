package public

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type DeviceStream struct {
	decoder *json.Decoder
	close   func() error
}

func (ds *DeviceStream) Receive() (string, ModelsDevice, error) {
	event := struct {
		Type  string       `json:"type"`
		Value ModelsDevice `json:"value"`
	}{}
	err := ds.decoder.Decode(&event)
	if err != nil {
		return "", event.Value, err
	}
	return event.Type, event.Value, nil
}

func (ds *DeviceStream) Close() error {
	return ds.close()
}

func (r ApiListDevicesInOrganizationRequest) Watch() (*DeviceStream, *http.Response, error) {
	return r.ApiService.ListDevicesInOrganizationWatch(r)
}

func (a *DevicesApiService) ListDevicesInOrganizationWatch(r ApiListDevicesInOrganizationRequest) (*DeviceStream, *http.Response, error) {
	var (
		localVarHTTPMethod = http.MethodGet
		localVarPostBody   interface{}
		formFiles          []formFile
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DevicesApiService.ListDevicesInOrganization")
	if err != nil {
		return nil, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/api/organizations/{organization_id}/devices"
	localVarPath = strings.Replace(localVarPath, "{"+"organization_id"+"}", url.PathEscape(parameterValueToString(r.organizationId, "organizationId")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	if r.gtRevision != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "gt_revision", r.gtRevision, "")
	}
	localVarQueryParams["watch"] = []string{"true"}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return nil, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return nil, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {

		localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
		localVarHTTPResponse.Body.Close()
		localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
		if err != nil {
			return nil, localVarHTTPResponse, err
		}

		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		if localVarHTTPResponse.StatusCode == 400 {
			var v ModelsBaseError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return nil, localVarHTTPResponse, newErr
			}
			newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
			newErr.model = v
			return nil, localVarHTTPResponse, newErr
		}
		if localVarHTTPResponse.StatusCode == 401 {
			var v ModelsBaseError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return nil, localVarHTTPResponse, newErr
			}
			newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
			newErr.model = v
			return nil, localVarHTTPResponse, newErr
		}
		if localVarHTTPResponse.StatusCode == 429 {
			var v ModelsBaseError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return nil, localVarHTTPResponse, newErr
			}
			newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
			newErr.model = v
			return nil, localVarHTTPResponse, newErr
		}
		if localVarHTTPResponse.StatusCode == 500 {
			var v ModelsBaseError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return nil, localVarHTTPResponse, newErr
			}
			newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
			newErr.model = v
		}
		return nil, localVarHTTPResponse, newErr
	}

	return &DeviceStream{
		close:   localVarHTTPResponse.Body.Close,
		decoder: json.NewDecoder(localVarHTTPResponse.Body),
	}, localVarHTTPResponse, nil
}

// Informer creates a *ApiListDevicesInOrganizationInformer which provides a simpler
// API to list devices but which is implemented with the Watch api.  The *ApiListDevicesInOrganizationInformer
// maintains a local device cache which gets updated with the Watch events.
func (r ApiListDevicesInOrganizationRequest) Informer() *ApiListDevicesInOrganizationInformer {
	res := &ApiListDevicesInOrganizationInformer{
		request:        r,
		modifiedSignal: make(chan struct{}, 1),
	}
	return res
}

type ApiListDevicesInOrganizationInformer struct {
	request        ApiListDevicesInOrganizationRequest
	stream         *DeviceStream
	inSync         chan struct{}
	modifiedSignal chan struct{}
	mu             sync.RWMutex
	data           map[string]ModelsDevice
	response       *http.Response
	err            error
	lastRevision   int32
}

var ErrContextCanceled = errors.New("context canceled")

func (s *ApiListDevicesInOrganizationInformer) Changed() <-chan struct{} {
	return s.modifiedSignal
}

func (s *ApiListDevicesInOrganizationInformer) Execute() (map[string]ModelsDevice, *http.Response, error) {

	var err error
	s.mu.Lock()
	if s.stream == nil {
		// after an error we can recover by resuming event's from the last revision.
		s.request.GtRevision(s.lastRevision)
		s.stream, s.response, s.err = s.request.ApiService.ListDevicesInOrganizationWatch(s.request)
		err = s.err
		if s.err == nil {
			s.inSync = make(chan struct{})
			go s.readStream(s.lastRevision)
		}
	}
	s.mu.Unlock()

	// initial api request may have failed...
	if err != nil {
		return s.data, s.response, s.err
	}

	// avoid returning a partial data list by, waiting for the bookmark event
	// which signals that all known data items have sent.  We wait for the inSync
	// chanel to close (or the context to be canceled).
	select {
	case <-s.request.ctx.Done():
		return s.data, s.response, ErrContextCanceled
	case <-s.inSync:
	}

	// s.data, s.response, s.err are modified with the s.mu write lock
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data, s.response, s.err
}

func (s *ApiListDevicesInOrganizationInformer) readStream(lastRevision int32) {
	isInSync := false

	defer func() {
		s.mu.Lock()
		err := s.stream.Close()
		if err != nil {
			s.err = err
		}
		s.stream = nil
		s.mu.Unlock()
		if !isInSync {
			isInSync = true
			close(s.inSync)
		}
	}()

	items := map[string]ModelsDevice{}
	for {
		event, item, err := s.stream.Receive()
		if err != nil {
			s.setResult(nil, lastRevision, err)
			return
		}
		switch event {
		case "change":
			lastRevision = item.Revision
			items[item.Id] = item
			if isInSync {
				s.setResult(dataByIdToByKey(items), lastRevision, nil)
			}
		case "delete":
			lastRevision = item.Revision
			delete(items, item.Id)
			if isInSync {
				s.setResult(dataByIdToByKey(items), lastRevision, nil)
			}
		case "bookmark":
			if !isInSync {
				isInSync = true
				s.setResult(dataByIdToByKey(items), lastRevision, nil)
				close(s.inSync)
			}
		case "close":
			return
		case "error":
			return
		default:
			s.setResult(nil, lastRevision, fmt.Errorf("unknown event type: %s", event))
			return
		}
	}
}

func dataByIdToByKey(dataById map[string]ModelsDevice) map[string]ModelsDevice {
	// Convert to a map of public keys to devices
	data := map[string]ModelsDevice{}
	for _, i := range dataById {
		data[i.PublicKey] = i
	}
	return data
}

func (s *ApiListDevicesInOrganizationInformer) setResult(data map[string]ModelsDevice, lastRevision int32, err error) {
	s.mu.Lock()
	s.data = data
	s.err = err
	s.lastRevision = lastRevision
	s.mu.Unlock()

	select {
	// try to signal...
	case s.modifiedSignal <- struct{}{}:
	default: // so we don't block if a signal is pending.
	}
}
