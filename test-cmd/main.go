package main

import (
	"fmt"
	"log"
	"net/http"
)

const usersJSON = `{
  "data": [
    {
      "id": "usr_101",
      "name": "Amara Okonkwo",
      "email": "amara.okonkwo@gmail.com",
      "phone": "+234-803-456-7890",
      "ssn": "123-45-6789",
      "date_of_birth": "1990-03-15",
      "address": {
        "street": "14 Admiralty Way",
        "city": "Lagos",
        "state": "Lagos",
        "country": "Nigeria",
        "zip": "101233"
      },
      "role": "customer",
      "account_status": "active",
      "created_at": "2022-01-10T08:30:00Z",
      "region": "NG"
    },
    {
      "id": "usr_102",
      "name": "Chidi Eze",
      "email": "chidi.eze@yahoo.com",
      "phone": "+234-701-234-5678",
      "ssn": "987-65-4321",
      "date_of_birth": "1985-07-22",
      "address": {
        "street": "22 Herbert Macaulay Way",
        "city": "Abuja",
        "state": "FCT",
        "country": "Nigeria",
        "zip": "900001"
      },
      "role": "customer",
      "account_status": "active",
      "created_at": "2021-06-18T14:00:00Z",
      "region": "NG"
    },
    {
      "id": "usr_103",
      "name": "Sarah Mitchell",
      "email": "sarah.mitchell@outlook.com",
      "phone": "+1-555-234-5678",
      "ssn": "456-78-9012",
      "date_of_birth": "1992-11-05",
      "address": {
        "street": "89 Oak Street",
        "city": "Austin",
        "state": "TX",
        "country": "US",
        "zip": "73301"
      },
      "role": "customer",
      "account_status": "suspended",
      "created_at": "2023-03-01T09:15:00Z",
      "region": "US"
    },
    {
      "id": "usr_104",
      "name": "Fatima Al-Hassan",
      "email": "fatima.alhassan@proton.me",
      "phone": "+234-812-345-6789",
      "ssn": "321-54-9876",
      "date_of_birth": "1988-02-28",
      "address": {
        "street": "5 Kano Road",
        "city": "Kano",
        "state": "Kano",
        "country": "Nigeria",
        "zip": "700001"
      },
      "role": "premium_customer",
      "account_status": "active",
      "created_at": "2020-09-12T11:00:00Z",
      "region": "NG"
    }
  ],
  "meta": {
    "total": 4
  }
}`

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok", "service": "auvis-test-server"}`))
	})

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(usersJSON))
	})

	port := ":8000"
	fmt.Printf("⚡️ High-performance Go mock upstream running on http://localhost%s\n", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
