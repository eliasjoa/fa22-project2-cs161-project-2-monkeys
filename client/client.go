package client

// CS 161 Project 2

// You MUST NOT change these default imports. ANY additional imports
// may break the autograder!

import (
	"encoding/json"

	userlib "github.com/cs161-staff/project2-userlib"
	"github.com/google/uuid"

	// hex.EncodeToString(...) is useful for converting []byte to string

	// Useful for string manipulation
	"strings"

	// Useful for formatting strings (e.g. `fmt.Sprintf`).
	"fmt"

	// Useful for creating new error messages to return using errors.New("...")
	"errors"

	// Optional.
	_ "strconv"
)

// This serves two purposes: it shows you a few useful primitives,
// and suppresses warnings for imports not being used. It can be
// safely deleted!
func someUsefulThings() {

	// Creates a random UUID.
	randomUUID := uuid.New()

	// Prints the UUID as a string. %v prints the value in a default format.
	// See https://pkg.go.dev/fmt#hdr-Printing for all Golang format string flags.
	userlib.DebugMsg("Random UUID: %v", randomUUID.String())

	// Creates a UUID deterministically, from a sequence of bytes.
	hash := userlib.Hash([]byte("user-structs/alice"))
	deterministicUUID, err := uuid.FromBytes(hash[:16])
	if err != nil {
		// Normally, we would `return err` here. But, since this function doesn't return anything,
		// we can just panic to terminate execution. ALWAYS, ALWAYS, ALWAYS check for errors! Your
		// code should have hundreds of "if err != nil { return err }" statements by the end of this
		// project. You probably want to avoid using panic statements in your own code.
		panic(errors.New("An error occurred while generating a UUID: " + err.Error()))
	}
	userlib.DebugMsg("Deterministic UUID: %v", deterministicUUID.String())

	// Declares a Course struct type, creates an instance of it, and marshals it into JSON.
	type Course struct {
		name      string
		professor []byte
	}

	course := Course{"CS 161", []byte("Nicholas Weaver")}
	courseBytes, err := json.Marshal(course)
	if err != nil {
		panic(err)
	}

	userlib.DebugMsg("Struct: %v", course)
	userlib.DebugMsg("JSON Data: %v", courseBytes)

	// Generate a random private/public keypair.
	// The "_" indicates that we don't check for the error case here.
	var pk userlib.PKEEncKey
	var sk userlib.PKEDecKey
	pk, sk, _ = userlib.PKEKeyGen()
	userlib.DebugMsg("PKE Key Pair: (%v, %v)", pk, sk)

	// Here's an example of how to use HBKDF to generate a new key from an input key.
	// Tip: generate a new key everywhere you possibly can! It's easier to generate new keys on the fly
	// instead of trying to think about all of the ways a key reuse attack could be performed. It's also easier to
	// store one key and derive multiple keys from that one key, rather than
	originalKey := userlib.RandomBytes(16)
	derivedKey, err := userlib.HashKDF(originalKey, []byte("mac-key"))
	if err != nil {
		panic(err)
	}
	userlib.DebugMsg("Original Key: %v", originalKey)
	userlib.DebugMsg("Derived Key: %v", derivedKey)

	// A couple of tips on converting between string and []byte:
	// To convert from string to []byte, use []byte("some-string-here")
	// To convert from []byte to string for debugging, use fmt.Sprintf("hello world: %s", some_byte_arr).
	// To convert from []byte to string for use in a hashmap, use hex.EncodeToString(some_byte_arr).
	// When frequently converting between []byte and string, just marshal and unmarshal the data.
	//
	// Read more: https://go.dev/blog/strings

	// Here's an example of string interpolation!
	_ = fmt.Sprintf("%s_%d", "file", 1)
}

// This is the type definition for the User struct.
// A Go struct is like a Python or Java class - it can have attributes
// (e.g. like the Username attribute) and methods (e.g. like the StoreFile method below).
const key_length = 16

type User struct {
	Username              string
	Password              []byte
	Secret_key            userlib.PKEDecKey
	Signature_private_key userlib.DSSignKey
	master_key            []byte
	hmac_key              []byte
	Files_owned           map[string]bool
}

type File struct {
	Content   []byte
	Next_uuid uuid.UUID //UUID of the next file in list after this one
}

type FileController struct {
	Start uuid.UUID //UUID of first file
	End   uuid.UUID //UUID of end of file
}

type FileReferenceOwner struct {
	Uuid_shared_with        map[string]uuid.UUID
	Enc_keys_shared_with    map[string][]byte
	Hmac_keys_shared_with   map[string][]byte
	File_enc_key            []byte
	Hmac_key                []byte
	File_controller_pointer uuid.UUID //UUID for file controller
}

type FileReferencePrimary struct {
	File_enc_key            []byte
	Hmac_key                []byte
	File_controller_pointer uuid.UUID
}

type FileReferenceSecondary struct {
	File_Reference_Primary_enc_key []byte
	Hmac_key                       []byte
	HMAC                           []byte
	File_reference_primary_pointer uuid.UUID
}

type Invitation struct {
	FileReferencePrimary_decrypt_key []byte
	FileReferencePrimary_hmac_key    []byte
	FileReferencePrimary_pointer     uuid.UUID
}

// You can add other attributes here if you want! But note that in order for attributes to
// be included when this struct is serialized to/from JSON, they must be capitalized.
// On the flipside, if you have an attribute that you want to be able to access from
// this struct's methods, but you DON'T want that value to be included in the serialized value
// of this struct that's stored in datastore, then you can use a "private" variable (e.g. one that
// begins with a lowercase letter).

// NOTE: The following methods have toy (insecure!) implementations.

func InitUser(Username string, password string) (userdataptr *User, err error) {
	var userdata User
	if Username == "" {
		return nil, errors.New("Username cannot be nothing")
	}

	userdata.Username = Username
	password_salt := Username + "p"
	userdata.Password = userlib.Hash([]byte(password + password_salt))

	//Create public and private key
	var pk userlib.PKEEncKey
	var sk userlib.PKEDecKey
	pk, sk, err = userlib.PKEKeyGen()
	if err != nil {
		return nil, errors.New("Error in creating RSA key pair")
	}
	userdata.Secret_key = sk

	//Create digital signature keys
	var DS_sk userlib.DSSignKey
	var DS_pk userlib.DSVerifyKey
	DS_sk, DS_pk, err = userlib.DSKeyGen()
	if err != nil {
		return nil, errors.New("Error in creating RSA key pair for digital signature")
	}
	userdata.Signature_private_key = DS_sk

	//Create master key with enough entropy using PBKDF
	master_key_salt := Username + "k"
	master_key := userlib.Argon2Key([]byte(userdata.Password), []byte(master_key_salt), key_length)
	userdata.master_key = master_key

	//Create HMAC key using master key into HBKDF where only 16 bytes are needed as that is
	//the input size of the key in HMACEval
	HMAC_key_64, err := userlib.HashKDF(userdata.master_key, []byte("HMAC key for user"))
	if err != nil {
		return nil, errors.New("Error in generation of mac key for user")
	}

	HMAC_key := HMAC_key_64[:16]
	userdata.hmac_key = HMAC_key

	//Check if the user exists
	user_public_key_keystore := "Public key for:" + Username
	_, ok := userlib.KeystoreGet(user_public_key_keystore)
	if ok {
		return nil, errors.New("The user already exists")
	}

	//Put public keys in keystore, both signature and for encryption of invitation
	err = userlib.KeystoreSet(user_public_key_keystore, pk)
	if err != nil {
		return nil, errors.New("Could not put public key into keystore")
	}

	user_signature_key_keystore := "Signature key for:" + Username
	err = userlib.KeystoreSet(user_signature_key_keystore, DS_pk)
	if err != nil {
		return nil, errors.New("Could not put public key for signature into keystore")
	}

	//Now put the userdata into datastore where it is also encrypted
	//Use a hash of the Username as UUID
	//The UUID needs a 16 byte slice so use the first 16 bytes of the hash for the UUID
	b := userlib.Hash([]byte(Username))[:16]
	user_UUID, err := uuid.FromBytes(b)
	if err != nil {
		return nil, errors.New("Could not create a new value for an instance in the datastore for the user")
	}

	//Turn the data into JSON
	user_bytes, err := json.Marshal(userdata)
	if err != nil {
		return nil, errors.New("Could not Marshal the userdata into bytes")
	}

	//Encrypt it
	iv := userlib.RandomBytes(16)
	user_bytes_encrypted := userlib.SymEnc(master_key, iv, user_bytes)

	//Then MAC it (64 bytes)
	HMAC, err := userlib.HMACEval(HMAC_key, user_bytes_encrypted)
	if err != nil {
		return nil, errors.New("Could not append the HMAC to userdata")
	}
	user_bytes_encrypted_MAC := append(user_bytes_encrypted, HMAC...)

	//Store it
	userlib.DatastoreSet(user_UUID, user_bytes_encrypted_MAC)

	return &userdata, nil
}

func GetUser(Username string, password string) (userdataptr *User, err error) {
	var userdata User
	userdataptr = &userdata

	//Update userdata value with given Username and password
	userdata.Username = Username
	password_salt := Username + "p"
	userdata.Password = userlib.Hash([]byte(password + password_salt))

	//Check whether user exists using the Keystore
	user_public_key_keystore := "Public key for:" + Username
	_, ok := userlib.KeystoreGet(user_public_key_keystore)
	if ok == false {
		return nil, errors.New("The user does not exists")
	}

	//Finds the UUID
	b := userlib.Hash([]byte(Username))[:16]
	user_UUID, err := uuid.FromBytes(b)
	if err != nil {
		return nil, errors.New("Could find the user in the datastore")
	}

	//Retrieve data
	userdata_bytes_encrypted_mac, ok := userlib.DatastoreGet(user_UUID)
	if !ok {
		return nil, errors.New("No data found for that UUID")
	}

	//Splice HMAC and the encrypted json data
	userdata_bytes_encrypted := userdata_bytes_encrypted_mac[:len(userdata_bytes_encrypted_mac)-64]
	HMAC := userdata_bytes_encrypted[len(userdata_bytes_encrypted):]

	//Compute the HMAC on the encrypted json data to check integrity
	//Need to compute HMAC key

	//Recompute master key with enough entropy using PBKDF
	master_key_salt := Username + "k"
	master_key := userlib.Argon2Key([]byte(userdata.Password), []byte(master_key_salt), key_length)
	userdata.master_key = master_key

	//Recompute HMAC key using master key into HBKDF where only 16 bytes are needed as that is
	//the input size of the key in HMACEval
	HMAC_key_64, err := userlib.HashKDF(userdata.master_key, []byte("HMAC key for user"))
	if err != nil {
		return nil, errors.New("Error in generation of mac key for user")
	}

	HMAC_key := HMAC_key_64[:16]

	//Check if this HMAC_key computes the same HMAC as the one stored in datastore
	new_HMAC, err := userlib.HMACEval(HMAC_key, userdata_bytes_encrypted)
	if err != nil {
		return nil, errors.New("Could not compute the HMAC using the new HMAC key")
	}

	equal := userlib.HMACEqual(HMAC, new_HMAC)
	if equal {
		return nil, errors.New("HMAC tag is wrong, integrity of userdata not verified")
	}

	//We can now decrypt the data as the HMAC is verified
	userdata_bytes := userlib.SymDec(master_key, userdata_bytes_encrypted)

	//Unmarshal the userdata
	err = json.Unmarshal(userdata_bytes, userdataptr)
	if err != nil {
		return nil, err
	}

	return userdataptr, nil
}

func (userdata *User) StoreFile(filename string, content []byte) (err error) {
	//First check if the file exists

	//To do this we need to compute the UUID
	//Password is already stores as a hash
	var file_uuid_bytes []byte
	file_uuid_bytes = append(file_uuid_bytes, userlib.Hash([]byte(userdata.Username))...)
	file_uuid_bytes = append(file_uuid_bytes, userdata.Password...)
	file_uuid_bytes = append(file_uuid_bytes, userlib.Hash([]byte(filename))...)
	file_uuid, err := uuid.FromBytes(userlib.Hash(file_uuid_bytes)[:16])
	if err != nil {
		return err
	}
	//Check if file exists
	datastore_file_content, ok := userlib.DatastoreGet(file_uuid)

	//Create the encryption key
	encryption_key_64, err := userlib.HashKDF(userdata.master_key, []byte("Encryption key for file"+filename))
	if err != nil {
		return err
	}
	encryption_key := encryption_key_64[:16]
	//Create the HMAC key
	hmac_key_64, err := userlib.HashKDF(userdata.master_key, []byte("HMAC key for file"+filename))
	if err != nil {
		return err
	}
	hmac_key := hmac_key_64[:16]

	//If the file does not exist
	if !ok {
		//Update the userdata with the new file owned
		userdata.Files_owned[filename] = true
		//Upload the new userdata to the datastore using helper function
		err := UploadUserdata(userdata)
		if err != nil {
			return err
		}

		//Create all the new structs needed for creating a file and the keys
		//needed to encrypt file and filecontroller
		var file_reference_owner FileReferenceOwner

		//Keys for file encryption and HMAC
		file_reference_owner.File_enc_key = userlib.RandomBytes(16)
		file_reference_owner.Hmac_key = userlib.RandomBytes(16)
		//The new file's UUID
		file_reference_owner.File_controller_pointer = uuid.New()

		//The sharing functionality
		//Create a map for sharing of the UUID, HMAC_keys and Encryption keys to people with access
		file_reference_owner.Uuid_shared_with = make(map[string]uuid.UUID)
		file_reference_owner.Hmac_keys_shared_with = make(map[string][]byte)
		file_reference_owner.Enc_keys_shared_with = make(map[string][]byte)

		//Turn filereferenceowner to bytes and encrypt and then mac
		file_reference_owner_bytes, err := json.Marshal(file_reference_owner)
		if err != nil {
			return err
		}

		//Encrypt
		iv := userlib.RandomBytes(16)
		file_reference_owner_bytes_encrypted := userlib.SymEnc(encryption_key, iv, file_reference_owner_bytes)

		//MAC
		filereferenceowner_HMAC, err := userlib.HMACEval(hmac_key, file_reference_owner_bytes_encrypted)
		if err != nil {
			return err
		}
		file_reference_owner_bytes_encrypted_HMAC := append(file_reference_owner_bytes_encrypted, filereferenceowner_HMAC...)

		//Store filereferenceowner in datastore with Frombytes(username + password + filename) as uuid
		userlib.DatastoreSet(file_uuid, file_reference_owner_bytes_encrypted_HMAC)

		//Now we can create the file and the file controller
		var file File
		var file_controller FileController

		file_controller.Start = uuid.New()
		file_controller.End = uuid.New()

		file.Content = content
		file.Next_uuid = file_controller.End

		//Now the we have a file controller keeping track of where the file linked lists starts and ends
		//We then store the filecontroller at the uuid referenced by the filereferenceowner

		//Marshal file controller
		file_controller_bytes, err := json.Marshal(file_controller)
		if err != nil {
			return err
		}
		//Encrypt file controller
		iv = userlib.RandomBytes(16)
		file_controller_bytes_encrypted := userlib.SymEnc(file_reference_owner.File_enc_key, iv, file_controller_bytes)
		//HMAC it
		file_controller_bytes_HMAC, err := userlib.HMACEval(file_reference_owner.Hmac_key, file_controller_bytes_encrypted)
		if err != nil {
			return err
		}
		file_controller_bytes_encrypted_HMAC := append(file_controller_bytes_encrypted, file_controller_bytes_HMAC...)
		//Store in datastore
		userlib.DatastoreSet(file_reference_owner.File_controller_pointer, file_controller_bytes_encrypted_HMAC)

		//Now we store the file at the UUID referenced by file_controller.Start
		//Marshall it
		file_bytes, err := json.Marshal(file)
		if err != nil {
			return err
		}
		//Encrypt it using filereferenceowner key
		iv = userlib.RandomBytes(16)
		file_bytes_encrypted := userlib.SymEnc(file_reference_owner.File_enc_key, iv, file_bytes)
		//HMAC it
		file_bytes_HMAC, err := userlib.HMACEval(file_reference_owner.Hmac_key, file_bytes_encrypted)
		if err != nil {
			return err
		}
		file_bytes_encrypted_HMAC := append(file_bytes_encrypted, file_bytes_HMAC...)
		//Send the file to the datastore at the uuid of file_controller.start
		userlib.DatastoreSet(file_controller.Start, file_bytes_encrypted_HMAC)

		//Finally, add an empty file to the end of the linked list (file_controller.End)
		var empty_file File

		//Marshall it
		empty_file_bytes, err := json.Marshal(empty_file)
		if err != nil {
			return err
		}
		//Encrypt it
		iv = userlib.RandomBytes(16)
		empty_file_bytes_encrypted := userlib.SymEnc(file_reference_owner.File_enc_key, iv, empty_file_bytes)
		//HMAC it
		empty_file_bytes_HMAC, err := userlib.HMACEval(file_reference_owner.Hmac_key, empty_file_bytes_encrypted)
		if err != nil {
			return err
		}
		empty_file_bytes_encrypted_HMAC := append(empty_file_bytes_encrypted, empty_file_bytes_HMAC...)
		//Store it in datastore at file_controller.End uuid
		userlib.DatastoreSet(file_controller.End, empty_file_bytes_encrypted_HMAC)

		return nil
	}
	//For when the file already exists
	//If the user owns the file
	owns_file := userdata.Files_owned[filename]
	if owns_file {
		var file_reference_owner FileReferenceOwner
		var file_controller FileController

		//We want to now access the filereference owner to see where our filecontroller is
		//Need to first check integrity of filereferenceowner using valid mac helper function
		if !ValidHMAC(hmac_key, datastore_file_content) {
			return errors.New("The integrity of the FileReferenceOwner of this file has been compromised")
		}
		//As it is integral we can load the filereference owner from datastore_file_content
		datastore_file_content_encrypted := datastore_file_content[:len(datastore_file_content)-64]
		//Decrypt it
		datastore_file_content_decrypted := userlib.SymDec(encryption_key, datastore_file_content_encrypted)
		//Unmarshal it
		err := json.Unmarshal(datastore_file_content_decrypted, &file_reference_owner)
		if err != nil {
			return err
		}
		//Load the file controller
		file_controller_bytes_with_HMAC, ok := userlib.DatastoreGet(file_reference_owner.File_controller_pointer)
		if !ok {
			return errors.New("Something wrong with accessing file controller")
		}
		//Check HMAC of file controller
		if !ValidHMAC(file_reference_owner.Hmac_key, file_controller_bytes_with_HMAC) {
			return errors.New("The integrity of the file controller is compromised")
		}
		//Decrypt it
		file_controller_bytes_encrypted := file_controller_bytes_with_HMAC[:len(file_controller_bytes_with_HMAC)-64]
		file_controller_bytes_decrypted := userlib.SymDec(file_reference_owner.File_enc_key, file_controller_bytes_encrypted)
		//Demarshal it
		err = json.Unmarshal(file_controller_bytes_decrypted, &file_controller)
		if err != nil {
			return err
		}

		//Now we know where the start and end of the linked file is
		//The idea is to create a new file and update the start attribute in the file controller
		//The empty end of the previous file list can be discarded as there are no requirement that this should be deleted

		//Now we create the new start and end to the linked list of files
		var start_file File
		var end_file File
		start_file.Content = content
		//Add the next file as a random new uuid
		start_file.Next_uuid = uuid.New()
		//Update the file_controller with the new tail
		file_controller.End = start_file.Next_uuid
		//Store the new file at the beginning of the list as told in the file_controller.start
		//Marshal
		start_file_bytes, err := json.Marshal(start_file)
		if err != nil {
			return err
		}
		//Encrypt
		iv := userlib.RandomBytes(16)
		start_file_bytes_encrypted := userlib.SymEnc(file_reference_owner.File_enc_key, iv, start_file_bytes)
		//HMAC
		start_file_bytes_HMAC, err := userlib.HMACEval(file_reference_owner.Hmac_key, start_file_bytes_encrypted)
		if err != nil {
			return err
		}
		start_file_bytes_encrypted_HMAC := append(start_file_bytes_encrypted, start_file_bytes_HMAC...)
		//Store it at file_controller_start
		userlib.DatastoreSet(file_controller.Start, start_file_bytes_encrypted_HMAC)

		//Do the same for the empty tail
		//Marshal
		end_file_bytes, err := json.Marshal(end_file)
		if err != nil {
			return err
		}
		//Encrypt
		iv = userlib.RandomBytes(16)
		end_file_bytes_encrypted := userlib.SymEnc(file_reference_owner.File_enc_key, iv, end_file_bytes)
		//HMAC
		end_file_bytes_HMAC, err := userlib.HMACEval(file_reference_owner.Hmac_key, end_file_bytes_encrypted)
		if err != nil {
			return err
		}
		end_file_bytes_encrypted_HMAC := append(end_file_bytes_encrypted, end_file_bytes_HMAC...)
		//Store it at file_controller_end or file_starts next uuid
		userlib.DatastoreSet(start_file.Next_uuid, end_file_bytes_encrypted_HMAC)

		return nil
	}
	//If the user does not own the file
	//We now take advantage of our filereferenceprimary and filereferencesecondary structs
	var file_reference_primary FileReferencePrimary
	var file_reference_secondary FileReferenceSecondary
	var file_controller FileController
	//We first open the filereferencesecondary file
	//This would be at the UUID inputted when calling store file
	//That is: FromBytes(username+password+filename)
	//This is already loaded
	//Check HMAC
	if !ValidHMAC(hmac_key, datastore_file_content) {
		return errors.New("Integrity of the filereferencesecondary is compromised")
	}
	//If HMAC is valid decrypt and then demarshal
	//Decrypt using the key used to encrypt the filereferencesecondary upon sharing
	datastore_file_content_encrypted := datastore_file_content[:len(datastore_file_content)-64]
	datastore_file_content_decrypted := userlib.SymDec(encryption_key, datastore_file_content_encrypted)
	//Demarshal
	err = json.Unmarshal(datastore_file_content_decrypted, &file_reference_secondary)
	if err != nil {
		return err
	}

	//Now we can open the filereferenceprimary from the filereferencesecondary
	file_reference_primary_bytes_encrypted_HMAC, ok := userlib.DatastoreGet(file_reference_secondary.File_reference_primary_pointer)
	if !ok {
		return errors.New("Not able to retrieve the filereferenceprimary file from filereferencesecondary file")
	}
	//Check the HMCA of the file_reference_primary
	if !ValidHMAC(file_reference_secondary.Hmac_key, file_reference_primary_bytes_encrypted_HMAC) {
		return errors.New("The integrity of the filereferenceprimary is compromised")
	}
	//Decrypt it
	file_reference_primary_bytes_encrypted := file_reference_primary_bytes_encrypted_HMAC[:len(file_reference_primary_bytes_encrypted_HMAC)-64]
	file_reference_primary_bytes_decrypted := userlib.SymDec(file_reference_secondary.File_Reference_Primary_enc_key, file_reference_primary_bytes_encrypted)
	//Unmarshal
	err = json.Unmarshal(file_reference_primary_bytes_decrypted, &file_reference_primary)
	if err != nil {
		return err
	}
	//We now have access to the filereferenceprimary which gives access to the file controller and the encryption key and hmac key
	//We can now load the file controller
	file_controller_bytes_encrypted_HMAC, ok := userlib.DatastoreGet(file_reference_primary.File_controller_pointer)
	if !ok {
		return errors.New("Could not retrieve file controller from filereferenceprimary")
	}
	//Check the HMAC
	if !ValidHMAC(file_reference_primary.Hmac_key, file_controller_bytes_encrypted_HMAC) {
		return errors.New("The integrity of the file controller has been compromised")
	}
	//Decrypt
	file_controller_bytes_encrypted := file_controller_bytes_encrypted_HMAC[:len(file_controller_bytes_encrypted_HMAC)-64]
	file_controller_bytes_decrypted := userlib.SymDec(file_reference_primary.File_enc_key, file_controller_bytes_encrypted)
	//Unmarshal
	err = json.Unmarshal(file_controller_bytes_decrypted, &file_controller)
	if err != nil {
		return err
	}

	//Now we have access to the file controller and can store the new content
	var start_file File
	var end_file File
	start_file.Content = content
	start_file.Next_uuid = uuid.New()
	//Update the end of the file list
	file_controller.End = start_file.Next_uuid

	//Store the start file
	//Marshal
	start_file_bytes, err := json.Marshal(start_file)
	if err != nil {
		return err
	}
	//Encrypt
	iv := userlib.RandomBytes(16)
	start_file_bytes_encrypted := userlib.SymEnc(file_reference_primary.File_enc_key, iv, start_file_bytes)
	//Add HMAC
	start_file_bytes_HMAC, err := userlib.HMACEval(file_reference_primary.Hmac_key, start_file_bytes_encrypted)
	if err != nil {
		return err
	}
	start_file_bytes_encrypted_HMAC := append(start_file_bytes_encrypted, start_file_bytes_HMAC...)
	//Store in datastore
	userlib.DatastoreSet(file_controller.Start, start_file_bytes_encrypted_HMAC)

	//Do the same for the empty tail
	//Marshal
	end_file_bytes, err := json.Marshal(end_file)
	if err != nil {
		return err
	}
	//Encrypt
	iv = userlib.RandomBytes(16)
	end_file_bytes_encrypted := userlib.SymEnc(file_reference_primary.File_enc_key, iv, end_file_bytes)
	//HMAC
	end_file_bytes_HMAC, err := userlib.HMACEval(file_reference_primary.Hmac_key, end_file_bytes_encrypted)
	if err != nil {
		return err
	}
	end_file_bytes_encrypted_HMAC := append(end_file_bytes_encrypted, end_file_bytes_HMAC...)
	//Store it at file_controller_end or file_starts next uuid
	userlib.DatastoreSet(start_file.Next_uuid, end_file_bytes_encrypted_HMAC)

	return nil
}

//Might have to reupload file controller to datastore, not sure yet

func (userdata *User) AppendToFile(filename string, content []byte) error {
	return nil
}

func (userdata *User) LoadFile(filename string) (content []byte, err error) {
	storageKey, err := uuid.FromBytes(userlib.Hash([]byte(filename + userdata.Username))[:16])
	if err != nil {
		return nil, err
	}
	dataJSON, ok := userlib.DatastoreGet(storageKey)
	if !ok {
		return nil, errors.New(strings.ToTitle("file not found"))
	}
	err = json.Unmarshal(dataJSON, &content)
	return content, err
}

func (userdata *User) CreateInvitation(filename string, recipientUsername string) (
	invitationPtr uuid.UUID, err error) {
	return
}

func (userdata *User) AcceptInvitation(senderUsername string, invitationPtr uuid.UUID, filename string) error {
	return nil
}

func (userdata *User) RevokeAccess(filename string, recipientUsername string) error {
	return nil
}

func UploadUserdata(userdata *User) (err error) {
	//Now put the userdata into datastore where it is also encrypted
	//Use a hash of the Username as UUID
	//The UUID needs a 16 byte slice so use the first 16 bytes of the hash for the UUID
	b := userlib.Hash([]byte(userdata.Username))[:16]
	user_UUID, err := uuid.FromBytes(b)
	if err != nil {
		return nil
	}

	//Turn the data into JSON
	user_bytes, err := json.Marshal(userdata)
	if err != nil {
		return nil
	}

	//Encrypt it
	iv := userlib.RandomBytes(16)
	user_bytes_encrypted := userlib.SymEnc(userdata.master_key, iv, user_bytes)

	//Then MAC it (64 bytes)
	HMAC, err := userlib.HMACEval(userdata.hmac_key, user_bytes_encrypted)
	if err != nil {
		return nil
	}
	user_bytes_encrypted_MAC := append(user_bytes_encrypted, HMAC...)

	//Store it
	userlib.DatastoreSet(user_UUID, user_bytes_encrypted_MAC)

	return nil
}

func ValidHMAC(hmac_key []byte, content_with_HMAC []byte) (valid bool) {
	hmac := content_with_HMAC[len(content_with_HMAC)-64:]
	content := content_with_HMAC[0 : len(content_with_HMAC)-64]

	hmac_content, err := userlib.HMACEval(hmac_key, content)
	if err != nil {
		return false
	}
	valid = userlib.HMACEqual(hmac, hmac_content)
	return valid
}
