package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// Define ProductDB (for database / value integrity purpose)
// Use int64 to avoid financial floating point rounding / calculation loss (in case needed in the future)
// Example: 10000 in the database represents 100.00 in calculation or user interaction
type ProductDB struct {
	Id          int64
	Name        string
	Description string
	Price       int64
}

// Since golang doesn't have native Decimal implementation
// There is a good library, https://pkg.go.dev/github.com/shopspring/decimal
// But we just need small portion of the lib feature
// So we can implement custom type instead based on float64
type Decimal float64

// Define ProductAPI (for API interaction purpose)
// Data input from API/user, price in Decimal -> int64(price * 100) -> convert to ProductDB model
// Vice versa for data output to the API
// Include data validation for some fields
// https://pkg.go.dev/github.com/go-playground/validator/v10
type ProductAPI struct {
	Id          *int64  `json:"id"` // optional, use *int64 to differentiate between missing fields and zero
	Name        string  `json:"name" validate:"required,excludesall=<>\"'\\%&*:;?@^{}[]~$=!0x7C0x2C"`
	Description string  `json:"description" validate:"required,excludesall=<>\"'\\*?@^{}[]~=!0x7C"`
	Price       Decimal `json:"price" validate:"required,gt=0"`
}

// Override MarshalJSON() method for Decimal type
// If we don't override the method, the decimal places will be omitted in JSON for the case below
// "xxx.00" will only show "xxx", because only zeros on decimal places
// https://pkg.go.dev/encoding/json
// https://www.tiredsg.dev/blog/override-golang-json-marshal/
func (d *Decimal) MarshalJSON() ([]byte, error) {
	// https://pkg.go.dev/strconv#FormatFloat
	// Convert the price in float with 2 digit in decimal places
	priceDecimal := strconv.FormatFloat(float64(*d), 'f', 2, 64)
	// Return as slice of byte to omit " symbol (numeric format instead of string)
	return []byte(priceDecimal), nil
}

// Data conversion model from ProductAPI to ProductDB
func APItoDB(p *ProductAPI) ProductDB {
	return ProductDB{
		Id:          lastId,
		Name:        p.Name,
		Description: p.Description,
		Price:       int64(p.Price * 100),
	}
}

// Data conversion model from ProductDB to ProductAPI
func DBtoAPI(p *ProductDB) ProductAPI {
	return ProductAPI{
		Id:          &p.Id,
		Name:        p.Name,
		Description: p.Description,
		Price:       Decimal(float64(p.Price) / 100),
	}
}

// Define custom data validator
// https://echo.labstack.com/docs/request#validate-data
type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	err := cv.validator.Struct(i)
	if err != nil {
		// return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		return err
	}
	return nil
}

// Declare products dummy data and lastId for data id / indexing purpose
var products []ProductDB
var lastId int64

// Response data model
type ResponseJSON struct {
	Message string `json:"message"`
}

// Main function yuhuuu
func main() {
	// Dummy data initialization
	products = []ProductDB{
		{
			Id:          1,
			Name:        "Galaxy Z Fold6",
			Description: "The ultimate foldable powered by Galaxy AI",
			Price:       189999,
		},
		{
			Id:          2,
			Name:        "Galaxy Z Flip6",
			Description: "The power of Galaxy AI right in your pocket",
			Price:       110000,
		},
		{
			Id:          3,
			Name:        "Galaxy S24 Ultra",
			Description: "The new era of AI-enhanced smartphones",
			Price:       129900,
		},
		{
			Id:          4,
			Name:        "Galaxy Watch Ultra",
			Description: "Galaxy AI is here",
			Price:       64999,
		},
	}
	// Dummy indicator
	lastId = int64(len(products))

	// Create new echo instance
	// https://echo.labstack.com/docs/quick-start
	e := echo.New()

	// Bind custom validator that created before
	// https://echo.labstack.com/docs/request#validate-data
	e.Validator = &CustomValidator{validator: validator.New()}

	// Healthcheck endpoint
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "")
	})

	// Create Product
	e.POST("/api/products", createProductHandler)

	// Get Product by ID
	e.GET("/api/products/:id", getProductByIdHandler)

	// Get All Products
	e.GET("/api/products", getAllProductsHandler)

	// Update Prodcut by ID
	e.PUT("/api/products/:id", updateProductHandler)

	// Delete Product by ID
	e.DELETE("/api/products/:id", deleteProductHandler)

	e.Logger.Fatal(e.Start("localhost:8080"))
}

func validateProductAPI(c echo.Context, newProduct ProductAPI) error {
	// Validate data format
	err := c.Validate(newProduct)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "ProductAPI.Name"):
			return echo.NewHTTPError(http.StatusBadRequest, "Bad request, 'name' field required and shouldn't contains <>\"'\\%&*:;?@^{}[]~$=!|, symbols")
		case strings.Contains(err.Error(), "ProductAPI.Description"):
			return echo.NewHTTPError(http.StatusBadRequest, "Bad request, 'description' field required and shouldn't contains <>\"'\\*?@^{}[]~=!| symbols")
		case strings.Contains(err.Error(), "ProductAPI.Price"):
			return echo.NewHTTPError(http.StatusBadRequest, "Bad request, 'price' field required and should be larger than 0")
		}
	}
	return nil
}

func createProductHandler(c echo.Context) error {
	newProduct := new(ProductAPI)
	// Bind request body to declared struct / data model
	err := c.Bind(newProduct)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad request, failed to parse JSON")
	}
	// Validate data format
	err = validateProductAPI(c, *newProduct)
	if err != nil {
		return err
	}
	// Increment dummy data id indexing
	lastId++
	// Convert to ProductDB model
	product := APItoDB(newProduct)
	// Add to the database (slice)
	products = append(products, product)
	// Response data
	r := &ResponseJSON{
		Message: fmt.Sprintf("%s successfully created", newProduct.Name),
	}
	return c.JSON(http.StatusCreated, r)
}

func getProductByIdHandler(c echo.Context) error {
	id := c.Param("id")
	// convert path param from string to int64
	// https://pkg.go.dev/strconv#ParseInt
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		r := &ResponseJSON{
			Message: "Invalid Product ID",
		}
		return c.JSON(http.StatusBadRequest, r)
	}
	// Check product existence
	for _, product := range products {
		if product.Id == idInt {
			return c.JSON(http.StatusOK, product)
		}
	}
	// Response data
	r := &ResponseJSON{
		Message: fmt.Sprintf("Product with ID=%s not found", id),
	}
	return c.JSON(http.StatusNotFound, r)
}

func getAllProductsHandler(c echo.Context) error {
	// create slice of ProductAPI with specified length based on products count
	responseProducts := make([]ProductAPI, len(products))
	// convert from ProductDB to ProductAPI for each product
	for i, product := range products {
		responseProducts[i] = DBtoAPI(&product)
	}
	return c.JSON(http.StatusOK, responseProducts)
}

func updateProductHandler(c echo.Context) error {
	id := c.Param("id")
	// convert path param from string to int64
	// https://pkg.go.dev/strconv#ParseInt
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		r := &ResponseJSON{
			Message: "Invalid Product ID",
		}
		return c.JSON(http.StatusBadRequest, r)
	}
	// Check product existence
	for i, product := range products {
		if product.Id == idInt {
			newProduct := new(ProductAPI)
			// Bind request body to declared struct / data model
			err := c.Bind(newProduct)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Bad request, failed to parse JSON")
			}
			// Validate data format
			err = validateProductAPI(c, *newProduct)
			if err != nil {
				return err
			}
			// Update to new data
			newProductDB := ProductDB{
				Id:          product.Id,
				Name:        newProduct.Name,
				Description: newProduct.Description,
				Price:       int64(newProduct.Price * 100),
			}
			products[i] = newProductDB
			// Response data
			r := &ResponseJSON{
				Message: fmt.Sprintf("%s successfully updated", newProduct.Name),
			}
			return c.JSON(http.StatusOK, r)
		}
	}
	// Response data
	r := &ResponseJSON{
		Message: fmt.Sprintf("Product with ID=%s not found", id),
	}
	return c.JSON(http.StatusNotFound, r)
}

func deleteProductHandler(c echo.Context) error {
	id := c.Param("id")
	// convert path param from string to int64
	// https://pkg.go.dev/strconv#ParseInt
	idToRemove, err := strconv.ParseInt(id, 2, 64)
	if err != nil {
		r := &ResponseJSON{
			Message: "Invalid Product ID",
		}
		return c.JSON(http.StatusBadRequest, r)
	}

	var tempName string

	// Check product existence
	for i, product := range products {
		if product.Id == idToRemove {
			tempName = product.Name
			products = append(products[:i], products[i+1:]...)
			// Response data
			r := &ResponseJSON{
				Message: fmt.Sprintf("%s successfully deleted", tempName),
			}
			return c.JSON(http.StatusOK, r)
		}
	}
	// Response data
	r := &ResponseJSON{
		Message: fmt.Sprintf("Product with ID=%s not found", idToRemove),
	}
	return c.JSON(http.StatusNotFound, r)
}
