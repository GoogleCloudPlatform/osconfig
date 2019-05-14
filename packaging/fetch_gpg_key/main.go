//  Copyright 2019 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

// The application fetches encrypted certificate from GCS bucket,
// decrypts the certificate using cloudkms and stores it in /tmps
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"

	cloudkms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/storage"
	"github.com/kylelemons/godebug/pretty"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

// The values are used for testing only. The values will be replaced with actual ones
// before merging the commit
var (
	dump = &pretty.Config{IncludeUnexported: true}

	gcsBucket = "signing-certificate"
	gcsObject = "RPM-GPG-KEY-subrat.encrypted"

	projectID   = flag.String("gcs_project", "subratp-project", "GCS project where the certificate is stored")
	keyRingID   = flag.String("key_ring", "subratp-testing-gpg", "key ring used for decryption")
	keyName     = flag.String("key_name", "rpm-gpg", "key name used for decryption")
	keyLocation = flag.String("location", "global", "GCP location where the kms key is stored")
	kmsKeyName  = fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s", *projectID, *keyLocation, *keyRingID, *keyName)
	outFileName = flag.String("out_path", "/tmp/RPM-GPG-KEY-subrat-decrypted", "output file path name")
)

func main() {
	ctx := context.Background()
	fmt.Println("reading encrpyted file...")
	dat, err := readFromGcs(ctx, gcsBucket, gcsObject)
	check(err)
	fmt.Printf("decrypting file...\n")
	ddata, err := decrypt(ctx, kmsKeyName, dat)
	check(err)
	fmt.Println("writing data to file")
	writeFile(ddata, "/tmp/RPM-GPG-KEY-subrat-decrypted")
	fmt.Println("stored decrypted signature file")
}

func writeFile(data *[]byte, outFilePath string) {
	err := ioutil.WriteFile(outFilePath, *data, 0400)
	check(err)
}

func readFromGcs(ctx context.Context, bucket, object string) (*[]byte, error) {

	client, err := storage.NewClient(ctx)
	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func decrypt(ctx context.Context, keyName string, ciphertext *[]byte) (*[]byte, error) {
	client, err := cloudkms.NewKeyManagementClient(ctx)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		return nil, err
	}

	// Build the request.
	req := &kmspb.DecryptRequest{
		Name:       keyName,
		Ciphertext: *ciphertext,
	}
	fmt.Printf("request: %s\n", req)
	// Call the API.
	response, err := client.Decrypt(ctx, req)
	fmt.Printf("response: %s\n", dump.Sprint(nil))
	if err != nil {
		return nil, fmt.Errorf("decryption request failed: %+v", err)
	}
	return &response.Plaintext, nil
}

func check(e error) {
	if e != nil {
		fmt.Printf("%v\n", e)
		panic(e)
	}
}
