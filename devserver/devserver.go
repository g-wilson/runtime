package devserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/g-wilson/runtime"
	ctxtool "github.com/g-wilson/runtime/context"
	"github.com/g-wilson/runtime/hand"
	"github.com/g-wilson/runtime/logger"
	"github.com/g-wilson/runtime/rpcservice"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/sirupsen/logrus"
)

// Server is our dev server instance
type Server struct {
	ListenAddress string
	Log           *logrus.Entry

	authMiddleware func(next http.Handler) http.Handler
	r              *chi.Mux
}

// New creates a dev server
func New(addr string) *Server {
	log := logger.Create("debug", "text", "debug")

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.AllowContentType("application/json"))

	s := &Server{
		ListenAddress: addr,
		Log:           log,
		r:             r,
	}

	return s
}

// AddService maps an RPC Service's methods to HTTP path on the server's router
func (s *Server) AddService(path string, svc *rpcservice.Service, authnr *Authenticator) *Server {
	s.r.Route(fmt.Sprintf("/%s", path), func(r chi.Router) {
		r.Use(attachRequestLogger(svc.Logger))

		if authnr != nil {
			r.Use(newAuthenticationMiddleware(authnr, svc.IdentityProvider))
		}

		r.Options("/*", optionsHandler)

		for name, method := range svc.Methods {
			r.Post("/"+name, wrapRPCMethod(svc, method))
		}
	})

	return s
}

// Listen starts listening for HTTP requests and blocks unless it panics
func (s *Server) Listen() {
	s.Log.Infof("runtime dev server listening on %q\n", s.ListenAddress)

	if err := http.ListenAndServe(s.ListenAddress, s.r); err != http.ErrServerClosed {
		s.Log.Fatalf("Could not listen on %q: %s\n", s.ListenAddress, err)
	}
}

func attachRequestLogger(logInstance *logrus.Entry) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(logger.SetContext(r.Context(), logInstance))

			reqLogger := logger.FromContext(r.Context())

			reqLogger.Update(reqLogger.Entry().WithFields(logrus.Fields{
				"request_id": middleware.GetReqID(r.Context()),
			}))

			next.ServeHTTP(w, r)
			return
		})
	}
}

func newAuthenticationMiddleware(authenticator *Authenticator, identityProvider rpcservice.IdentityContextProvider) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("authorization")
			if token == "" {
				err := hand.New("authentication_required")
				logger.FromContext(r.Context()).Entry().WithError(err).Error("request failed")
				sendHTTPError(w, err)
				return
			}

			claims, err := authenticator.Authenticate(r.Context(), token)
			if err != nil {
				reqLogger := logger.FromContext(r.Context())
				reqLogger.Entry().WithError(fmt.Errorf("jwt authenticator failed: %w", err)).Error("request failed")
				sendHTTPError(w, err)
				return
			}

			ctx := identityProvider(r.Context(), claims)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
			return
		})
	}
}

func wrapRPCMethod(svc *rpcservice.Service, method *rpcservice.Method) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := ctxtool.SetRequestID(r.Context(), middleware.GetReqID(r.Context()))
		reqLogger := logger.FromContext(ctx)

		setCORSHeaders(w)

		if r.Body == nil {
			http.Error(w, runtime.ErrCodeMissingBody, http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			sendHTTPError(w, hand.New(runtime.ErrCodeInvalidBody))
			return
		}

		for _, fn := range svc.ContextProviders {
			ctx = fn(ctx)
		}

		result, err := method.Invoke(ctx, body)
		if err != nil {
			sendHTTPError(w, err)
			return
		}

		if result == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		resBytes, err := json.Marshal(result)
		if err != nil {
			reqLogger.Entry().WithError(err).Error("encoding response failed")
			sendHTTPError(w, hand.New(runtime.ErrCodeUnknown))
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(resBytes)
	}
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE,GET,HEAD,PUT,POST,PATCH,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Host,Origin,Accept")
}

func optionsHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

func sendHTTPError(w http.ResponseWriter, err error) {
	var status int

	handErr, ok := err.(hand.E)
	if !ok {
		handErr = hand.New(runtime.ErrCodeUnknown)
	}

	switch handErr.Code {
	case runtime.ErrCodeBadRequest:
		fallthrough
	case runtime.ErrCodeInvalidBody:
		fallthrough
	case runtime.ErrCodeSchemaFailure:
		fallthrough
	case runtime.ErrCodeMissingBody:
		status = http.StatusBadRequest

	case runtime.ErrCodeForbidden:
		status = http.StatusForbidden

	case runtime.ErrCodeNoAuthentication:
		fallthrough
	case runtime.ErrCodeInvalidAuthentication:
		status = http.StatusUnauthorized

	default:
		status = http.StatusInternalServerError
	}

	body, err := json.Marshal(handErr)
	if err != nil {
		body = []byte(`{"code":"error_serialisation_fail"}`)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
}
