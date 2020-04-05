package devserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/auth"
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

	authMiddleware func(next http.Handler) http.Handler
	log            *logrus.Entry
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
	r.Use(attachRequestLogger(log))

	s := &Server{
		ListenAddress: addr,
		log:           log,
		r:             r,
	}

	return s
}

// WithAuthenticator configures the server with an auth middleware for validating JWTs
func (s *Server) WithAuthenticator(authnr *Authenticator) *Server {
	s.authMiddleware = newAuthenticationMiddleware(authnr)
	return s
}

// AddService maps an RPC Service's methods to HTTP path on the server's router
func (s *Server) AddService(path string, svc *rpcservice.Service, authenticator bool) *Server {
	s.r.Route(fmt.Sprintf("/%s", path), func(r chi.Router) {
		r.Options("/*", optionsHandler)

		if authenticator {
			r.Use(s.authMiddleware)
		}

		for name, method := range svc.Methods {
			r.Post("/"+name, wrapRPCMethod(method))
		}
	})

	return s
}

// Listen starts listening for HTTP requests and blocks unless it panics
func (s *Server) Listen() {
	log.Printf("runtime dev server listening on %q\n", s.ListenAddress)

	if err := http.ListenAndServe(s.ListenAddress, s.r); err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %q: %s\n", s.ListenAddress, err)
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

func newAuthenticationMiddleware(authenticator *Authenticator) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("authorization")
			if token == "" {
				sendHTTPError(w, hand.New("authentication_required"))
				return
			}

			credentials, err := authenticator.Authenticate(r.Context(), token)
			if err != nil {
				sendHTTPError(w, err)
				return
			}

			r = r.WithContext(auth.SetIdentityContext(r.Context(), *credentials))

			next.ServeHTTP(w, r)
			return
		})
	}
}

func optionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE,GET,HEAD,PUT,POST,PATCH,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Host,Origin,Accept")
	w.WriteHeader(204)
}

func wrapRPCMethod(method *rpcservice.Method) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqLogger := logger.FromContext(r.Context())

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

		result, err := method.Invoke(r.Context(), body)
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

		sendHTTPResponse(w, resBytes, http.StatusOK)
	}
}

func sendHTTPResponse(w http.ResponseWriter, body []byte, status int) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE,GET,HEAD,PUT,POST,PATCH,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Host,Origin,Accept")
	w.Header().Set("Content-Type", "application/json; charset=utf8")
	w.WriteHeader(status)
	w.Write(body)
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

	errBytes, err := json.Marshal(handErr)
	if err != nil {
		errBytes = []byte(`{"code":"error_serialisation_fail"}`)
	}

	sendHTTPResponse(w, errBytes, status)
}
