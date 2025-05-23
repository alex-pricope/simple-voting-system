### 🚀 Demo

- [Voting UI (use any code)](https://alex-pricope.github.io/simple-voting-system/demo-ui/index.html)
- [Results Page](https://alex-pricope.github.io/simple-voting-system/demo-ui/results.html)

## Why this?
Recently I offered my help in organizing the current company's yearly hackathon and the team asked me to deal with the voting system.
I wanted to move away from the classic _Excel spreadsheet + Google Form_.

## What is it?
This is a simple,_yet challenging_, project I decided to implement in my spare time to deal with the voting problem. 

The idea was to create a **simple voting system**
* Only **1 vote per person** - using a unique `voting code` to stop dupe voting
* Have a very `simple and intuitive` UI
* Each code has a `weight` - this was a requirement: the `grand_jury` vote has more weight than the `other_team` or `general_public`
* Another requirement was for _people to vote for all teams, and rate them under categories_
  * So, I decided to use a `5-star` system for each team for each category
  * This allows me to apply weights to each category - _maybe presentation is more important than fun?_

The simple UI (not connected to any API and with code validation disabled) can be found here(todo - link)

## The API
The voting UI uses a few simple API requests that should be public.
* `POST : /api/vote` - register a new vote
* `GET : /api/codes/{code}` - validate the code before voting 
* `GET : /api/results` - display the results
* `GET : /api/meta/categories` - get the categories for the voting
* `GET : /api/meta/teams` - get the teams and details
![image](https://github.com/user-attachments/assets/b11d65d6-b6ee-42d2-91ca-2eb3d69e0d7d)


On top of these, I decided to create some private endpoints - available only in `swagger` running from my machine on the day of voting.
These are only here to do maintenance and manual intervention and should not be on a critical path. 

Also, in a real-world scenario, these should be more protected (having only a simple API key right now, as an example).

Of course, a real API should be versioned as well.

### Admin Group
The admin group is used to manage the initial voting codes.
* `POST : /api/admin/codes` - private - create new codes (bulk) by category
* `GET : /api/admin/codes` - private - get all the codes
* `GET : /api/admin/categories` - private - get all the voting categories
* `GET : /api/admin/codes/{category}` - private - get all the codes for category
* `DELETE : /api/admin/codes/{code}` - private - delete a specific code
* `POST : /api/admin/codes/{code}/reset` - private - reset a code to unused
* `POST : /api/admin/codes/reset` - private - reset all codes to unused
* `DELETE : /api/admin/votes` - private - delete all votes
![image](https://github.com/user-attachments/assets/58774c00-bfc0-4c3e-a875-c9acc5fed8b6)


### Meta: Voting categories
The Voting Categories are used to manage the categories where the teams will be voted for. This is displayed on the UI.
* `GET : /api/meta/categories` - public - (used by UI)
* `GET : /api/meta/categories/{ID}` - private - get all categories by ID
* `POST : /api/meta/categories` - private - create a new voting category
* `PUT : /api/meta/categories/{ID}` - private - update a voting category by ID
* `DELETE : /api/meta/categories/{ID}` - private - delete a voting category by ID
![image](https://github.com/user-attachments/assets/6fde6af7-5476-41c9-82d7-485d602df4cc)


### Meta: Teams
The Teams group is used to manage the teams (and its members)
* `GET : /api/meta/teams` - public - (used by UI)
* `GET : /api/meta/teams/{ID}` - private - get a team by ID
* `POST : /api/meta/teams` - private - create a team
* `PUT : /api/meta/teams/{ID}` - private - update a team
* `DELETE : /api/meta/teams/{ID}` - private - delete a team
![image](https://github.com/user-attachments/assets/3798d1b1-4789-4410-a126-405167678886)


## AWS
To keep the costs at a minimum and because I am dealing with around 100 - 200 burst requests I decided to use the following components
* `Route53` - already had a domain hosted here
* `CloudFront` and `S3` - to host the static element. 
* `DynamoDB` tables with very simple structure
  * _VotingCodes_ - holds the actual voting codes, has string PK on the code itself
  * _VotingCategories_ - holds the voting categories, ID int PK
  * _Teams_ - same as above but for teams
  * _Votes_ - a bit more complicated table, PK string with voting code, and a composite SK(SortKey)
    * `SortKey:    fmt.Sprintf("cat#%d#team#%d", v.CategoryID, v.TeamID),`
    * `PK: voting code`
    * This way, it is very easy to retrieve values
    * (add image)
* `Lambda function` to host the API written in `golang`
  * The lambda should be sufficient for approximately 200 burst requests
  * I picked ARM runtime because it is the cheapest
  * _Alternative: SQS queue + second lambda_ - This allows for a decoupled approach, introduces retries. Overkill for this exercise. 
* `API Gateway` with Lambda integration to expose the API inside the program.

```mermaid
graph TD
  R53[Route 53 domain] --> CF[CloudFront]
  CF --> S3[S3 - Static Frontend]
  CF --> APIGW[API Gateway route: /api/proxy+]
  APIGW --> Lambda[Lambda  - Go API, ARM64 runtime]

  Lambda --> DDBCodes[VotesCodes Dynamo Table]
  Lambda --> DDBCategories[Categories Dynamo Table]
  Lambda --> DDBTeams[Teams Dyanmo Table]
  Lambda --> DDBVotes[Votes Dynamo Table]

  subgraph Private
    S3
  end

  subgraph Backend
    APIGW
    Lambda
    DDBCodes
    DDBCategories
    DDBTeams
    DDBVotes
  end
```

## The code
The project, while could be a lot better, is simple 
* An `api package` that contains _controllers_, _models_, _transport_ and some _config_. 
  * There are integration tests inside
* A `logging package` because I like to log things
* A `storage package` to deal with repositories and dynamo impl. 
* `Swagger docs` for all endpoints - I want to run this locally to set things up.
* A `docker-compose` file
  * The idea is to use `localstack` to run the integration tests (and not using AWS credits)
  * Using `localstack + docker` you can run dynamo, S3, etc, locally in docker
  * More about localstack [here](https://docs.docker.com/guides/localstack/)
* The docker compose file also uses the `init-localstack.sh` file that does 2 simple things
  * Creates the needed dynamo tables
  * Creates a needed S3 bucket 
  * Waits for everything to be ready before finishing
* Lastly, a `makefile` to help with automation
  * For example, the lambda uses an `ARM runtime`, the makefile has a target inside to create the required zip file
  * Check the file for additional targets


## How to run this locally?
Use the makefile targets, for example, `make run-local` will build, generate swagger, start localstack (needs docker running), run the tests, start the app. 

## UI
The solution uses 2 simple html pages that uses the data from some of the API calls
* `/static/index.html` - landing page for the voting process, takes in a voting code
* `/static/results.html` - results page that shows the voting results

## Load Testing

To validate the system's performance under pressure, I wrote a simple load test using [k6](https://k6.io/). The goal was to simulate 100 users submitting votes concurrently, verifying that voting remained consistent, and that no double submissions or code re-use occurred.

### How to run it

1. Install `k6` if you don't have it:
   ```bash
   brew install k6
   ```

2. Navigate to the `loadtesting/` folder (or wherever you saved the script) and run:
   ```bash
   k6 run load-test.js
   ```

The test script (`loadtesting/load-tests.js`) sends randomized but valid votes, logging errors and ensuring the system behaves correctly under load.
## The vote process

There are 3 categories of voters:
- `grand_jury` with a weight of **0.5**
- `other_team` with a weight of **0.3**
- `general_public` with a weight of **0.2**

Each vote is evaluated across multiple voting categories, each with its own weight:
- `presentation` with a weight of **0.5**
- `innovation` with a weight of **0.4**
- `fun` with a weight of **0.1**

All voters are required to vote on **all teams** and **all categories**, ensuring consistent input and fair weight distribution across all votes.

---

## Voting Score Calculation

Each vote is cast by a user for every team in every category.

Each individual vote contributes a **weighted score** based on:

```
weighted_score = rating × voter_weight × category_weight
```
Where:
- `rating` is from 1 to 5 (selected by the voter)
- `voter_weight` depends on the voter group (e.g., `grand_jury = 0.5`)
- `category_weight` is the importance of the category (e.g., `presentation = 0.5`)

---

### Example

Assume:
- Voter A is from `grand_jury` (weight: 0.5)
- Category weights:
  - Innovation = 0.4
  - Presentation = 0.6

Voter A rates Team X:
- 4 stars in Innovation
- 5 stars in Presentation

```
Innovation score = 4 × 0.5 × 0.4 = 0.8
Presentation score = 5 × 0.5 × 0.6 = 1.5
Total team score from this voter = 0.8 + 1.5 = 2.3
```

If multiple voters cast votes, each team’s score per category is averaged across all voters, and the final team score is the **sum of category averages**.

Because we use **low raw weights (e.g. 0.2)**, the final scores are also small (e.g. between 0.3 and 0.8), but this is by design — only **relative rankings** matter.
