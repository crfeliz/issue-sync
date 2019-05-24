package lib

import (
	"fmt"
	"testing"
)

var simpleComment = `Comment [(ID 484163403)|https://github.com] from GitHub user [bilbo-baggins|https://github.com/bilbo-baggins] (Bilbo Baggins) at 16:27 PM, April 17 2019:

Bla blibidy bloo bla`

var complexCommentMetaData = "Comment [(ID 492248899)|https://github.com] from GitHub user [zeus|https://github.com/zeus] (Papa Zeus) at 13:57 PM, May 14 2019:"

var complexCommentBody = `Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do  @athen eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, 
quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. 
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. 
Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

On Tue, May 14, 2019 at 8:56 AM Demeter Harvester <notifications@github.com> wrote:

> Can you try again?
>
> [demeter@demeter the-planets (master)]$ docker pull olympus/the-planets:0.0.1-alpha2
> 0.0.1-alpha2: Pulling from pull olympus/the-planets
> // ....................
> Status: Downloaded newer image for pull olympus/the-planets:0.0.1-alpha2
>
> —
> You are receiving this because you authored the thread.
> Reply to this email directly, view it on GitHub
> <https://github.com/pull olympus/the-planets/issues/44?email_source=notifications&email_token=AAFITERB5I7BSKDSU2WWAXLPVLAJ7A5CNFSM4HMZ5SZKYY3PNVWWK3TUL52HS4DFVREXG43VMVBW63LNMVXHJKTDN5WW2ZLOORPWSZGODVLR27Y#issuecomment-492248447>,
> or mute the thread
> <https://github.com/notifications/unsubscribe-auth/AAFITESKFLMPNDSQS3P6GX3PVLAJ7ANCNFSM4HMZ5SZA>
> .
>
– 
Papa Zeus
Gender Pronouns: Zeu, Zeur, Zeurs
zeus@odyssey.com`

var complexComment = fmt.Sprintf("%s\n\n%s", complexCommentMetaData, complexCommentBody)


func TestJiraCommentRegexParsesSimpleComment(t *testing.T) {
	var fields = jCommentRegex.FindStringSubmatch(simpleComment)

	if len(fields) != 6 {
		t.Fatalf("Regex failed to parse fields %v", fields)
	}

	if fields[1] != "484163403" {
		t.Fatalf("Expected field[1] = 484163403; Got field[1] = %s", fields[1])
	}

	if fields[2] != "bilbo-baggins" {
		t.Fatalf("Expected field[2] = bilbo-baggins; Got field[2] = %s", fields[2])
	}

	if fields[3] != "Bilbo Baggins" {
		t.Fatalf("Expected field[3] = Bilbo Baggins; Got field[3] = %s", fields[3])
	}

	if fields[4] != "16:27 PM, April 17 2019" {
		t.Fatalf("Expected field[4] = 16:27 PM, April 17 2019; Got field[4] = %s", fields[4])
	}

	if fields[5] != "Bla blibidy bloo bla" {
		t.Fatalf("Expected field[5] = Bla blibidy bloo bla; Got field[5] = %s", fields[5])
	}
}

func TestJiraCommentRegexParsesSimpleCommentWithDashInUsername(t *testing.T) {
	var fields = jCommentRegex.FindStringSubmatch(complexComment)

	if len(fields) != 6 {
		t.Fatalf("Regex failed to parse fields %v", fields)
	}

	if fields[1] != "492248899" {
		t.Fatalf("Expected field[1] = 492248899; Got field[1] = %s", fields[1])
	}

	if fields[2] != "zeus" {
		t.Fatalf("Expected field[2] = zeus; Got field[2] = %s", fields[2])
	}

	if fields[3] != "Papa Zeus" {
		t.Fatalf("Expected field[3] = Papa Zeus; Got field[3] = %s", fields[3])
	}

	if fields[4] != "13:57 PM, May 14 2019" {
		t.Fatalf("Expected field[4] = 13:57 PM, May 14 2019; Got field[4] = %s", fields[4])
	}

	if fields[5] != complexCommentBody {
		t.Fatalf("Expected field[5] = Bla blibidy bloo bla; Got field[5] = %s", fields[5])
	}
}