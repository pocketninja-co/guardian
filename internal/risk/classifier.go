package risk

import (
	"math"
	"regexp"
	"strings"
)

// ClassifierType defines the predicted category of a document
type ClassifierType string

const (
	TypeMedical   ClassifierType = "Medical"
	TypeFinancial ClassifierType = "Financial"
	TypeGeneric   ClassifierType = "Generic"
)

// FeatureSet represents the "Bag of Words" with TF-IDF weights
type FeatureSet struct {
	MedicalWords   map[string]float64 // keyword -> TF-IDF weight
	FinancialWords map[string]float64
	MedicalBigrams   map[string]float64 // two-word phrases
	FinancialBigrams map[string]float64
}

type Classifier struct {
	features    FeatureSet
	phiPatterns []*regexp.Regexp // Rule-based PHI detection
}

func NewClassifier() *Classifier {
	c := &Classifier{
		features: FeatureSet{
			MedicalWords:     make(map[string]float64),
			FinancialWords:   make(map[string]float64),
			MedicalBigrams:   make(map[string]float64),
			FinancialBigrams: make(map[string]float64),
		},
	}
	c.train()
	c.compilePHIPatterns()
	return c
}

// train populates the feature set with TF-IDF weighted keywords
// TF-IDF (Term Frequency-Inverse Document Frequency) gives higher weight
// to words that are distinctive to a category vs common everywhere
func (c *Classifier) train() {
	// High-weight medical keywords (very distinctive)
	medicalHigh := []string{
		"patient", "diagnosis", "prescription", "physician", "hospital",
		"surgical", "pathology", "radiology", "oncology", "cardiology",
		"hipaa", "phi", "mrn", "medication", "procedure",
	}
	
	// Medium-weight medical keywords
	medicalMed := []string{
		"medical", "clinic", "treatment", "symptoms", "doctor", "nurse",
		"surgery", "anesthesia", "pediatric", "admitted", "discharged",
		"history", "rx", "insurance", "policy", "claim",
	}
	
	// High-weight financial keywords
	financialHigh := []string{
		"invoice", "payment", "transaction", "ledger", "revenue",
		"debit", "credit", "fiscal", "payroll", "receivable",
	}
	
	// Medium-weight financial keywords
	financialMed := []string{
		"bill", "amount", "due", "balance", "account", "bank",
		"statement", "audit", "tax", "profit", "loss", "quarter",
		"salary", "expense",
	}
	
	// Assign TF-IDF style weights
	for _, w := range medicalHigh {
		c.features.MedicalWords[w] = 3.0 // High importance
	}
	for _, w := range medicalMed {
		c.features.MedicalWords[w] = 1.5 // Medium importance
	}
	for _, w := range financialHigh {
		c.features.FinancialWords[w] = 3.0
	}
	for _, w := range financialMed {
		c.features.FinancialWords[w] = 1.5
	}
	
	// N-grams (Bigrams) - two-word phrases
	medicalBigrams := []string{
		"medical record", "patient history", "health information",
		"protected health", "medical history", "clinical notes",
		"prescription drug", "patient name", "date birth",
		"social security", "insurance number", "medical condition",
	}
	
	financialBigrams := []string{
		"bank account", "credit card", "account number",
		"payment method", "billing address", "purchase order",
		"financial statement", "tax id", "routing number",
	}
	
	for _, bg := range medicalBigrams {
		c.features.MedicalBigrams[bg] = 5.0 // Bigrams get high weight
	}
	for _, bg := range financialBigrams {
		c.features.FinancialBigrams[bg] = 5.0
	}
}

// compilePHIPatterns creates regex patterns for common PHI identifiers
func (c *Classifier) compilePHIPatterns() {
	patterns := []string{
		`\b\d{3}-\d{2}-\d{4}\b`,                    // SSN
		`\b[A-Z]{3}\d{6}\b`,                        // MRN (Medical Record Number)
		`\bDOB:?\s*\d{1,2}/\d{1,2}/\d{2,4}\b`,     // Date of Birth
		`\b(?:patient|pt)[\s#:]+\d+\b`,            // Patient ID
		`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`, // Credit Card
	}
	
	for _, p := range patterns {
		if re, err := regexp.Compile("(?i)" + p); err == nil {
			c.phiPatterns = append(c.phiPatterns, re)
		}
	}
}

// Classify analyzes text using TF-IDF weighted keywords, N-grams, and PHI patterns
func (c *Classifier) Classify(text string) (ClassifierType, float64) {
	// 1. Normalize text
	replacer := strings.NewReplacer(",", " ", "\t", " ", "\n", " ", "\r", " ", ".", " ", ";", " ", ":", " ")
	normalized := replacer.Replace(strings.ToLower(text))
	tokens := strings.Fields(normalized)
	
	if len(tokens) == 0 {
		return TypeGeneric, 0
	}
	
	medScore := 0.0
	finScore := 0.0
	
	// 2. Single-word matching with TF-IDF weights
	for _, token := range tokens {
		token = strings.Trim(token, "!?()[]{}'\"")
		
		if weight, ok := c.features.MedicalWords[token]; ok {
			medScore += weight
		}
		if weight, ok := c.features.FinancialWords[token]; ok {
			finScore += weight
		}
	}
	
	// 3. Bigram matching (N-grams)
	for i := 0; i < len(tokens)-1; i++ {
		bigram := tokens[i] + " " + tokens[i+1]
		
		if weight, ok := c.features.MedicalBigrams[bigram]; ok {
			medScore += weight
		}
		if weight, ok := c.features.FinancialBigrams[bigram]; ok {
			finScore += weight
		}
	}
	
	// 4. Rule-Based PHI Pattern Detection
	phiMatches := 0
	for _, pattern := range c.phiPatterns {
		if pattern.MatchString(text) {
			phiMatches++
		}
	}
	
	// Boost medical score significantly if PHI patterns found
	if phiMatches > 0 {
		medScore += float64(phiMatches) * 10.0
	}
	
	// 5. Calculate confidence
	totalScore := medScore + finScore
	if totalScore == 0 {
		return TypeGeneric, 0
	}
	
	if medScore > finScore {
		confidence := (medScore / totalScore) * 100
		
		// Apply logarithmic smoothing for high scores
		if medScore > 10 {
			confidence = 90 + (10 * math.Log10(medScore/10))
		}
		if confidence > 100 {
			confidence = 100
		}
		
		return TypeMedical, confidence
	} else {
		confidence := (finScore / totalScore) * 100
		if finScore > 10 {
			confidence = 90 + (10 * math.Log10(finScore/10))
		}
		if confidence > 100 {
			confidence = 100
		}
		return TypeFinancial, confidence
	}
}
