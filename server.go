// Copyright 2016 Mender Software AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.
package main

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/mendersoftware/go-lib-micro/config"
	"github.com/mendersoftware/go-lib-micro/log"
	"github.com/pkg/errors"

	api_http "github.com/mendersoftware/useradm/api/http"
	"github.com/mendersoftware/useradm/authz"
	"github.com/mendersoftware/useradm/jwt"
	"github.com/mendersoftware/useradm/keys"
	"github.com/mendersoftware/useradm/store/mongo"
	"github.com/mendersoftware/useradm/user"
)

func SetupAPI(stacktype string, authz authz.Authorizer, jwth jwt.JWTHandler) (*rest.Api, error) {
	api := rest.NewApi()
	if err := SetupMiddleware(api, stacktype, authz, jwth); err != nil {
		return nil, errors.Wrap(err, "failed to setup middleware")
	}

	//this will override the framework's error resp to the desired one:
	// {"error": "msg"}
	// instead of:
	// {"Error": "msg"}
	rest.ErrorFieldName = "error"

	return api, nil
}

func RunServer(c config.Reader) error {

	l := log.New(log.Ctx{})

	privKey, err := keys.LoadRSAPrivate(c.GetString(SettingPrivKeyPath))
	if err != nil {
		return errors.Wrap(err, "failed to read rsa private key")
	}

	authz := &SimpleAuthz{l: l}
	jwth := jwt.NewJWTHandlerRS256(privKey, l)

	useradmapi := api_http.NewUserAdmApiHandlers(
		func(l *log.Logger) (useradm.App, error) {
			db, err := mongo.GetDataStoreMongo(c.GetString(SettingDb))
			if err != nil {
				return nil, errors.Wrap(err, "database connection failed")
			}

			jwtHandler := jwth

			ua := useradm.NewUserAdm(jwtHandler, db, useradm.Config{
				Issuer:         c.GetString(SettingJWTIssuer),
				ExpirationTime: int64(c.GetInt(SettingJWTExpirationTimeout)),
			}, l)
			return ua, nil
		})

	api, err := SetupAPI(c.GetString(SettingMiddleware), authz, jwth)
	if err != nil {
		return errors.Wrap(err, "API setup failed")
	}

	apph, err := useradmapi.GetApp()
	if err != nil {
		return errors.Wrap(err, "inventory API handlers setup failed")
	}
	api.SetApp(apph)

	addr := c.GetString(SettingListen)
	l.Printf("listening on %s", addr)

	return http.ListenAndServe(addr, api.MakeHandler())
}
