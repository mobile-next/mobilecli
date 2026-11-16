package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

const (
	revokedBySecurity    = " (CSSMERR_TP_CERT_REVOKED)"
	notTrustedBySecurity = " (CSSMERR_TP_NOT_TRUSTED)"
	usable               = ""
)

var (
	nextYear = time.Now().AddDate(1, 0, 0)
	lastYear = time.Now().AddDate(-1, 0, 0)
)

// developerCertificate returns the DER bytes a provisioning profile stores for a certificate.
func developerCertificate(t *testing.T, name string, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    notAfter.AddDate(-1, 0, 0),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func hashOf(certificate []byte) string {
	sum := sha1.Sum(certificate)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// keychainLine is one identity as `security find-identity -v -p codesigning` prints it.
func keychainLine(certificate []byte, name, status string) string {
	return fmt.Sprintf(`  1) %s "%s"%s`, hashOf(certificate), name, status)
}

// keychainListing is the whole output of `security find-identity -v -p codesigning`.
func keychainListing(lines ...string) string {
	return strings.Join(lines, "\n") + fmt.Sprintf("\n     %d valid identities found\n", len(lines))
}

func profileWithCertificates(certificates ...[]byte) *provisioningProfile {
	return &provisioningProfile{Name: "Team Profile", DeveloperCertificates: certificates}
}

func assertNoUsableCertificate(t *testing.T, profile *provisioningProfile, keychain string, wantInError ...string) {
	t.Helper()
	identity, err := findProfileSigningIdentity(keychain, profile)
	if err == nil {
		t.Fatalf("chose identity %s, want an error", identity)
	}
	for _, want := range wantInError {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
}

func assertChosenIdentity(t *testing.T, profile *provisioningProfile, keychain string, wantCertificate []byte) {
	t.Helper()
	identity, err := findProfileSigningIdentity(keychain, profile)
	if err != nil {
		t.Fatal(err)
	}
	if identity != hashOf(wantCertificate) {
		t.Fatalf("chose identity %s, want %s", identity, hashOf(wantCertificate))
	}
}

func TestSigningUsesTheProfileCertificateThatIsInTheKeychain(t *testing.T) {
	valid := developerCertificate(t, "Dev Valid", nextYear)

	assertChosenIdentity(t, profileWithCertificates(valid), keychainListing(keychainLine(valid, "Dev Valid", usable)), valid)
}

func TestSigningSkipsARevokedCertificateAndUsesTheValidOne(t *testing.T) {
	revoked := developerCertificate(t, "Dev Revoked", nextYear)
	valid := developerCertificate(t, "Dev Valid", nextYear)
	keychain := keychainListing(keychainLine(revoked, "Dev Revoked", revokedBySecurity), keychainLine(valid, "Dev Valid", usable))

	assertChosenIdentity(t, profileWithCertificates(revoked, valid), keychain, valid)
	assertChosenIdentity(t, profileWithCertificates(valid, revoked), keychain, valid)
}

func TestSigningStopsWhenTheOnlyProfileCertificateIsRevoked(t *testing.T) {
	revoked := developerCertificate(t, "Dev Revoked", nextYear)
	keychain := keychainListing(keychainLine(revoked, "Dev Revoked", revokedBySecurity))

	assertNoUsableCertificate(t, profileWithCertificates(revoked), keychain, "Team Profile", "Dev Revoked", "revoked", "--provisioning-profile")
}

// security -v does not list an expired identity at all, so expiry is read from the certificate.
func TestSigningStopsWhenTheOnlyProfileCertificateIsExpired(t *testing.T) {
	expired := developerCertificate(t, "Dev Expired", lastYear)

	assertNoUsableCertificate(t, profileWithCertificates(expired), keychainListing(), "Dev Expired", "expired on "+lastYear.UTC().Format("2006-01-02"))
}

func TestSigningStopsWhenTheProfileCertificateIsNotInTheKeychain(t *testing.T) {
	absent := developerCertificate(t, "Dev Absent", nextYear)
	other := developerCertificate(t, "Dev Other", nextYear)
	keychain := keychainListing(keychainLine(other, "Dev Other", usable))

	assertNoUsableCertificate(t, profileWithCertificates(absent), keychain, "Dev Absent", "not in the keychain")
}

func TestSigningStopsOnAStatusItDoesNotKnow(t *testing.T) {
	untrusted := developerCertificate(t, "Dev Untrusted", nextYear)
	keychain := keychainListing(keychainLine(untrusted, "Dev Untrusted", notTrustedBySecurity))

	assertNoUsableCertificate(t, profileWithCertificates(untrusted), keychain, "Dev Untrusted", "CSSMERR_TP_NOT_TRUSTED")
}

func TestTheErrorListsEveryUnusableCertificateWithItsReason(t *testing.T) {
	revoked := developerCertificate(t, "Dev Revoked", nextYear)
	expired := developerCertificate(t, "Dev Expired", lastYear)
	absent := developerCertificate(t, "Dev Absent", nextYear)
	keychain := keychainListing(keychainLine(revoked, "Dev Revoked", revokedBySecurity))

	assertNoUsableCertificate(t, profileWithCertificates(revoked, expired, absent), keychain,
		"Dev Revoked ("+hashOf(revoked)[:8]+"): revoked",
		"Dev Expired ("+hashOf(expired)[:8]+"): expired on",
		"Dev Absent ("+hashOf(absent)[:8]+"): not in the keychain")
}

func TestSigningStopsWhenTheProfileHasNoCertificates(t *testing.T) {
	assertNoUsableCertificate(t, profileWithCertificates(), keychainListing(), "Team Profile")
}

func TestACertificateThatCannotBeParsedIsNamedByItsHash(t *testing.T) {
	garbage := []byte("not a certificate")

	assertNoUsableCertificate(t, profileWithCertificates(garbage), keychainListing(), hashOf(garbage)[:8], "not in the keychain")
}

const (
	testDevice   = "DEVICE-UDID"
	testBundleID = "com.example.app"
)

// candidateProfile is a development profile for the test device, as found in a profiles folder.
func candidateProfile(path, appID string, certificates ...[]byte) profileCandidate {
	return profileCandidate{path: path, profile: &provisioningProfile{
		Name:                  path,
		TeamIdentifier:        []string{"TEAM"},
		ProvisionedDevices:    []string{testDevice},
		DeveloperCertificates: certificates,
		Entitlements:          entitlements{GetTaskAllow: true, ApplicationIdentifier: appID},
		ExpirationDate:        nextYear,
	}}
}

func assertChosenProfile(t *testing.T, keychain string, wantPath string, candidates ...profileCandidate) {
	t.Helper()
	if got := chooseProfile(candidates, keychain, testDevice, testBundleID); got != wantPath {
		t.Fatalf("chose profile %q, want %q", got, wantPath)
	}
}

func TestProfileSelectionSkipsAProfileWhoseCertificatesAreRevoked(t *testing.T) {
	revoked := developerCertificate(t, "Dev Revoked", nextYear)
	valid := developerCertificate(t, "Dev Valid", nextYear)
	keychain := keychainListing(keychainLine(revoked, "Dev Revoked", revokedBySecurity), keychainLine(valid, "Dev Valid", usable))

	assertChosenProfile(t, keychain, "fresh.mobileprovision",
		candidateProfile("stale.mobileprovision", "TEAM.*", revoked),
		candidateProfile("fresh.mobileprovision", "TEAM.*", valid))
}

func TestProfileSelectionPrefersAUsableWildcardOverAnExactProfileItCannotSignWith(t *testing.T) {
	revoked := developerCertificate(t, "Dev Revoked", nextYear)
	valid := developerCertificate(t, "Dev Valid", nextYear)
	keychain := keychainListing(keychainLine(revoked, "Dev Revoked", revokedBySecurity), keychainLine(valid, "Dev Valid", usable))

	assertChosenProfile(t, keychain, "wildcard.mobileprovision",
		candidateProfile("exact.mobileprovision", "TEAM."+testBundleID, revoked),
		candidateProfile("wildcard.mobileprovision", "TEAM.*", valid))
}

func TestProfileSelectionStillPrefersAnExactProfileWhenBothAreUsable(t *testing.T) {
	valid := developerCertificate(t, "Dev Valid", nextYear)
	keychain := keychainListing(keychainLine(valid, "Dev Valid", usable))

	assertChosenProfile(t, keychain, "exact.mobileprovision",
		candidateProfile("wildcard.mobileprovision", "TEAM.*", valid),
		candidateProfile("exact.mobileprovision", "TEAM."+testBundleID, valid))
}

// with nothing usable, the best match is returned so signing can explain what is wrong with it.
func TestProfileSelectionFallsBackToTheBestMatchWhenNoneIsUsable(t *testing.T) {
	revoked := developerCertificate(t, "Dev Revoked", nextYear)
	keychain := keychainListing(keychainLine(revoked, "Dev Revoked", revokedBySecurity))

	assertChosenProfile(t, keychain, "exact.mobileprovision",
		candidateProfile("wildcard.mobileprovision", "TEAM.*", revoked),
		candidateProfile("exact.mobileprovision", "TEAM."+testBundleID, revoked))
}

func TestProfileSelectionFindsNothingWhenNoProfileMatches(t *testing.T) {
	valid := developerCertificate(t, "Dev Valid", nextYear)
	keychain := keychainListing(keychainLine(valid, "Dev Valid", usable))

	assertChosenProfile(t, keychain, "", candidateProfile("other.mobileprovision", "TEAM.com.example.other", valid))
}

func TestARevokedSigningIdentityIsRefusedByHashAndByName(t *testing.T) {
	revoked := developerCertificate(t, "Dev Revoked", nextYear)
	keychain := keychainListing(keychainLine(revoked, "Apple Development: Dev Revoked (ABC)", revokedBySecurity))

	for _, given := range []string{hashOf(revoked), strings.ToLower(hashOf(revoked)), "Apple Development: Dev Revoked (ABC)", "Dev Revoked"} {
		err := checkSigningIdentityOverride(given, keychain)
		if err == nil || !strings.Contains(err.Error(), "revoked") || !strings.Contains(err.Error(), hashOf(revoked)[:8]) {
			t.Errorf("--signing-identity %q: got %v, want an error that names the revoked certificate", given, err)
		}
	}
}

func TestAUsableOrUnknownSigningIdentityIsLeftToCodesign(t *testing.T) {
	revoked := developerCertificate(t, "Dev Shared Name", nextYear)
	valid := developerCertificate(t, "Dev Shared Name", nextYear)
	keychain := keychainListing(keychainLine(revoked, "Dev Shared Name", revokedBySecurity), keychainLine(valid, "Dev Shared Name", usable))

	for _, given := range []string{hashOf(valid), "Dev Shared Name", "Somebody Else"} {
		if err := checkSigningIdentityOverride(given, keychain); err != nil {
			t.Errorf("--signing-identity %q: unexpected error %v", given, err)
		}
	}
}

func TestProfileSelectionKeepsTheFirstOfTwoEquallyGoodProfiles(t *testing.T) {
	valid := developerCertificate(t, "Dev Valid", nextYear)
	keychain := keychainListing(keychainLine(valid, "Dev Valid", usable))

	assertChosenProfile(t, keychain, "first.mobileprovision",
		candidateProfile("first.mobileprovision", "TEAM.*", valid),
		candidateProfile("second.mobileprovision", "TEAM.*", valid))
}

func TestSecurityOutputIsReadAsIdentitiesWithTheirStatus(t *testing.T) {
	output := `  1) 1796b9ca80277c30dc5a2371d037954cd7f928f0 "Developer ID Application: Example UG (TEAM)"
  2) 9BFFF9D195A6592A01C2AD150E88419B48B358CB "Apple Development: Jane "JD" Doe (ABC)" (CSSMERR_TP_CERT_REVOKED)
     2 valid identities found
`
	want := []signingIdentity{
		{hash: "1796B9CA80277C30DC5A2371D037954CD7F928F0", name: "Developer ID Application: Example UG (TEAM)"},
		{hash: "9BFFF9D195A6592A01C2AD150E88419B48B358CB", name: `Apple Development: Jane "JD" Doe (ABC)`, problem: "CSSMERR_TP_CERT_REVOKED"},
	}

	got := parseSigningIdentities(output)
	if len(got) != len(want) {
		t.Fatalf("read %d identities, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("identity %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
