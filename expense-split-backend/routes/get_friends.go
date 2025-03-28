package routes

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/astaxie/beego/orm"
	"github.com/aws/aws-lambda-go/events"
)

// DashboardResponse struct to hold outstanding transactions
type DashboardResponse struct {
	FriendID   string  `json:"friend_id" orm:"column(friend_id)"`
	Name       string  `json:"name" orm:"column(name)"`
	NetBalance float64 `json:"netbalance" orm:"column(netbalance)"`
}

// GetDashboardHandler retrieves the dashboard data for a user
func GetDashboardHandler(o orm.Ormer, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	allowedOrigin := "http://easysplit.com:8080"

	userID := request.QueryStringParameters["user_id"]
	Mobile := request.QueryStringParameters["mobile"]

	if userID == "" && Mobile != "" {
		var foundUserID string
		err := o.Raw("SELECT user_id FROM userauth WHERE mobile = ?", Mobile).QueryRow(&foundUserID)
		if err != nil {
			log.Println("Error fetching user_id:", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Headers: map[string]string{
					"Access-Control-Allow-Origin":  allowedOrigin,
					"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
					"Access-Control-Allow-Headers": "Content-Type, Authorization",
					"Content-Type":                 "application/json",
				},
				Body: `{"message": "Invalid mobile number. User not found."}`,
			}, nil
		}
		userID = foundUserID
	}

	if userID == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Access-Control-Allow-Origin":  allowedOrigin,
				"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			}, Body: `{"message": "User ID is required"}`}, nil
	}

	var results []DashboardResponse

	query := `
    WITH balances AS (
        SELECT 
            PayeeID AS friend_id,
            SUM(CASE 
                	WHEN description = 'settle' THEN Amount 
                	ELSE Amount / 2 
            	END) AS amount
        FROM Global_transactions
        WHERE PayerID = ?
        GROUP BY PayeeID
        
        UNION ALL
        
        SELECT 
            PayerID AS friend_id,
            SUM(CASE 
                	WHEN description = 'settle' THEN -Amount 
                	ELSE -Amount / 2 
            	END) AS amount
        FROM Global_transactions
        WHERE PayeeID = ?
        GROUP BY PayerID
    )
    SELECT 
        g.friend_id,
        u.name,
        COALESCE(SUM(g.amount), 0) AS netbalance
    FROM balances g
    LEFT JOIN userauth u ON g.friend_id = u.user_id
    GROUP BY g.friend_id, u.name;
    `

	_, err := o.Raw(query, userID, userID).QueryRows(&results)
	if err != nil {
		log.Println("Database query error:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Access-Control-Allow-Origin":  allowedOrigin,
				"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
				"Content-Type":                 "application/json",
			}, Body: `{"message": "Failed to fetch dashboard data"}`}, nil
	}

	var totalbalance float64
	var Owedbalance float64

	for _, trs := range results {
		totalbalance += trs.NetBalance
		if trs.NetBalance > 0 {
			Owedbalance += trs.NetBalance
		}
	}

	var UserName string
	errr := o.Raw(`SELECT name FROM userauth WHERE user_id=?`, userID).QueryRow(&UserName)
	if errr != nil {
		log.Println("Error fetching user's name:", err)
		UserName = "Unknown" // Default if not found
	}

	responseBody, _ := json.Marshal(map[string]interface{}{
		"code":            200,
		"message":         "Dashboard data retrieved successfully",
		"data":            results,
		"userName":        UserName,
		"Balance":         totalbalance,
		"PositiveBalance": Owedbalance,
	})
	log.Printf("Query Results: %+v\n", results)
	log.Println("Userid:", userID)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Access-Control-Allow-Origin":  allowedOrigin,
			"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type, Authorization",
			"Content-Type":                 "application/json",
		},
		Body: string(responseBody),
	}, nil
}
