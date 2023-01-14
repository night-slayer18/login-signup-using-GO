package contollers

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	database "github.com/night-slayer18/golang-jwt-project/database"
	helper "github.com/night-slayer18/golang-jwt-project/helpers"
	"github.com/night-slayer18/golang-jwt-project/models"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"golang.org/x/crypto/bcrypt"
)

var userCollection *mongo.Collection = database.OpenCollection(database.Client, "user")
var validate = validator.New()

func HashPassword(password string) string {
	pass := []byte(password)
	hashedPassword, err := bcrypt.GenerateFromPassword(pass, 14)
	if err != nil {
		log.Panic(err)
	}
	return string(hashedPassword)
}

func VerifyPassword(userPassword string, providedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(providedPassword), []byte(userPassword))
	check := true
	//msg := ""

	if err != nil {
		//msg = fmt.Sprint("email or password is incorrect")
		check = false
	}
	return check
}
func Signup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		var user models.User
		defer cancel()
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		validatorErr := validate.Struct(user)
		if validatorErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": validatorErr.Error()})
			return
		}

		count, err := userCollection.CountDocuments(ctx, bson.M{"email": user.Email})
		defer cancel()
		if err != nil {
			log.Panic(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occured while checking for the email"})
		}
		if count > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "this email or phone already exists"})
		}

		password := HashPassword(*user.Password)
		user.Password = &password

		count, err = userCollection.CountDocuments(ctx, bson.M{"phone": user.Phone})
		defer cancel()
		if err != nil {
			log.Panic(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error occured while checking for the phone number"})
		}

		if count > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "this email or phone already exists"})
		}
		user.Created_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		user.Updated_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		user.ID = primitive.NewObjectID()
		user.User_id = user.ID.Hex()
		token, refreshToken, _ := helper.GenerateAllTokens(*user.Email, *user.First_name, *user.Last_name, *user.User_type, *&user.User_id)
		user.Token = &token
		user.Refresh_token = &refreshToken

		resultInsertionNumber, insertErr := userCollection.InsertOne(ctx, user)
		if insertErr != nil {
			// msg := fmt.Sprintf("User item was not created")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User item was not created"})
			return
		}
		defer cancel()
		c.JSON(http.StatusOK, resultInsertionNumber)
	}
}

func Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		var user models.User
		var foundUser models.User

		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err := userCollection.FindOne(ctx, bson.M{"email": user.Email}).Decode(&foundUser)
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "email or password is incorrect"})
			return
		}

		passwordIsValid := VerifyPassword(*user.Password, *foundUser.Password)
		defer cancel()
		if !passwordIsValid {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Password"})
			return
		}

		if foundUser.Email == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
			return
		}
		token, refreshToken, err := helper.GenerateAllTokens(*foundUser.Email, *foundUser.First_name, *foundUser.Last_name, *foundUser.User_type, *&foundUser.User_id)
		helper.UpdateAllTokens(token, refreshToken, *&foundUser.User_id)
		userCollection.FindOne(ctx, bson.M{"user_id": foundUser.User_id}).Decode(&foundUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, foundUser)
	}
}

func GetUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := helper.CheckUserType(c, "ADMIN")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)

		recordPerPage, err := strconv.Atoi(c.Query("recordPerPage"))
		if err != nil || recordPerPage < 1 {
			recordPerPage = 10
		}
		page, err1 := strconv.Atoi(c.Query("page"))
		if err1 != nil || page < 1 {
			page = 1
		}

		startIndex := (page - 1) * recordPerPage
		startIndex, _ = strconv.Atoi(c.Query("startIndex"))

		matchStage := bson.D{{Key: "$match", Value: bson.D{{}}}}
		groupStage := bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "_id", Value: "null"}}},
			{Key: "total_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "data", Value: bson.D{{Key: "$push", Value: "$$ROOT"}}}}}}
		projectStage := bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "_id", Value: 0},
				{Key: "total_count", Value: 1},
				{Key: "user_items", Value: bson.D{{Key: "slice", Value: []interface{}{"$data", startIndex, recordPerPage}}}}}}}
		result, err := userCollection.Aggregate(ctx, mongo.Pipeline{
			matchStage, groupStage, projectStage})
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error orccured while listing user items"})
		}
		var allUsers []bson.M
		if err = result.All(ctx, &allUsers); err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, allUsers[0])
	}
}

func GetUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.Param("user_id")

		if err := helper.MatchUserTypeToUid(c, userId); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)

		var user models.User
		result, err := userCollection.DeleteOne(ctx, bson.M{"user_id": userId})
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		fmt.Printf("DeleteOne removed %v document(s)\n", result.DeletedCount)
		c.JSON(http.StatusOK, user)
	}
}

// func deleteUser() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		userId := c.Param("user_id")

// 		if err := helper.MatchUserTypeToUid(c, userId); err != nil {
// 			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 			return
// 		}
// 		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)

// 		var user models.User
// 		result, err := userCollection.DeleteOne(ctx, bson.M{"user_id": userId})
// 		defer cancel()
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}
// 		fmt.Printf("DeleteOne removed %v document(s)\n", result.DeletedCount)
// 		c.JSON(http.StatusOK, user)
// 	}
// }

func UploadFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("image_name")
		formFile, err := c.FormFile("file")
		if err != nil {
			log.Fatal(err)
		}
		openedFile, err := formFile.Open()
		if err != nil {
			log.Fatal(err)
		}
		data, err := ioutil.ReadAll(openedFile)
		if err != nil {
			log.Fatal(err)
		}
		bucket, err := gridfs.NewBucket(
			database.Client.Database("myfiles"),
		)
		if err != nil {
			log.Fatal(err)
			return
		}
		uploadStream, err := bucket.OpenUploadStream(
			filename,
		)
		if err != nil {
			return
		}
		defer uploadStream.Close()
		fileSize, err := uploadStream.Write(data)
		if err != nil {
			log.Fatal(err)
			return
		}
		log.Printf("Write file to DB was successful. %s File size: %d M\n", filename, fileSize)
		c.JSON(http.StatusOK, uploadStream)
	}
}

func DownloadFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		fileName := c.Param("image_name")
		// For CRUD operations, here is an example
		db := database.Client.Database("myfiles")
		fsFiles := db.Collection("fs.files")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var results bson.M
		defer cancel()
		err := fsFiles.FindOne(ctx, bson.M{}).Decode(&results)
		if err != nil {
			log.Fatal(err)
			//log.Fatal("findone")
		}
		// you can print out the results
		//fmt.Println(results)

		bucket, _ := gridfs.NewBucket(
			db,
		)
		var buf bytes.Buffer
		dStream, err := bucket.DownloadToStreamByName(fileName, &buf)
		if err != nil {
			log.Fatal(err)
			//log.Fatal("download")
		}
		fmt.Printf("File size to download: %v\n", dStream)
		ioutil.WriteFile(fileName, buf.Bytes(), 0600)
		c.JSON(http.StatusOK, buf.Bytes())

		// img, _, err := image.Decode(bytes.NewReader(buf.Bytes()))
		// if err != nil {
		// 	log.Fatalln(err)
		// }
		// c.JSON(http.StatusOK, img)
	}
}
