import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import { Counter } from 'k6/metrics';

export const successVotes = new Counter('successful_votes');
export const duplicateVotes = new Counter('duplicate_votes');

const voteCodes = new SharedArray('codes', () => [
  "CODE001", "CODE002", "CODE003", "CODE004", "CODE005", "CODE006", "CODE007", "CODE008", "CODE009", "CODE010",
  "CODE011", "CODE012", "CODE013", "CODE014", "CODE015", "CODE016", "CODE017", "CODE018", "CODE019", "CODE020",
  "CODE021", "CODE022", "CODE023", "CODE024", "CODE025", "CODE026", "CODE027", "CODE028", "CODE029", "CODE030",
  "CODE031", "CODE032", "CODE033", "CODE034", "CODE035", "CODE036", "CODE037", "CODE038", "CODE039", "CODE040",
  "CODE041", "CODE042", "CODE043", "CODE044", "CODE045", "CODE046", "CODE047", "CODE048", "CODE049", "CODE050",
  "CODE051", "CODE052", "CODE053", "CODE054", "CODE055", "CODE056", "CODE057", "CODE058", "CODE059", "CODE060",
  "CODE061", "CODE062", "CODE063", "CODE064", "CODE065", "CODE066", "CODE067", "CODE068", "CODE069", "CODE070",
  "CODE071", "CODE072", "CODE073", "CODE074", "CODE075", "CODE076", "CODE077", "CODE078", "CODE079", "CODE080",
  "CODE081", "CODE082", "CODE083", "CODE084", "CODE085", "CODE086", "CODE087", "CODE088", "CODE089", "CODE090",
  "CODE091", "CODE092", "CODE093", "CODE094", "CODE095", "CODE096", "CODE097", "CODE098", "CODE099", "CODE100"
]);
const teams = [
  { id: 1, name: "Byte Bandits" },
  { id: 2, name: "404 Not Found" },
  { id: 3, name: "Null Terminators" },
  { id: 4, name: "Syntax Defenders" },
  { id: 5, name: "Infinite Loopers" },
  { id: 6, name: "Stack Smashers" },
  { id: 7, name: "Runtime Rebels" },
  { id: 8, name: "Packet Sniffers" },
  { id: 9, name: "Hackstreet Boys" },
];

const categories = [
  { id: 1, name: "Tech / Design / Innovation" },
  { id: 2, name: "Fun&Potential" },
  { id: 3, name: "Quality" },
  { id: 4, name: "Presentation" },
];

// 100 total users, 10 concurrent, exactly 100 vote codes
export const options = {
    vus: 100,
    iterations: 100,
};

export default function () {
    const code = voteCodes[__VU - 1];

    const votes = [];
    categories.forEach((cat) => {
      teams.forEach((team) => {
        votes.push({
          categoryId: cat.id,
          teamId: team.id,
          rating: Math.floor(Math.random() * 5) + 1 // random rating between 1 and 5
        });
      });
    });
    const payload = JSON.stringify({
        code: code,
        votes: votes,
    });

    const res = http.post('https://api.vote.qurl.ws/api/vote', payload, {
        headers: { 'Content-Type': 'application/json' },
    });
    console.log(`Sending vote request for code: ${code}`);
    //console.log(`Payload: ${payload}`);
    console.log(`Status code for ${code}: ${res.status}`);
    if (res.status !== 200) {
        console.log(`Non-200 response for code: ${code}`);
    if (res.status === 200) successVotes.add(1);
    else if (res.status === 409) duplicateVotes.add(1);
    }

    check(res, {
        'vote submitted or duplicate': r => r.status === 200 || r.status === 409,
    });
}