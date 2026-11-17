{
  "meta": {
    "cdf_version": 1,
    "title": "Sample: web server migration log",
    "subtitle": "A made-up change log showing the CDF row markers",
    "updated": "2026-11-17"
  },
  "sections": [
    {
      "id": "plan",
      "name": "Plan",
      "note": "Move the website from the old VM to the new host with no more than 10 minutes of downtime.",
      "rows": [
        { "date": "2026-11-03", "action": "MILESTONE: Migration approved", "detail": "Change window set for Saturday 02:00 to 04:00." },
        { "date": "2026-11-05", "action": "Build new host", "detail": "Same OS release, firewall rules copied from the old VM, TLS certificate installed." },
        { "date": "-", "action": "Decide on log retention", "detail": "Deferred: keep 30 days until the retention policy is agreed." }
      ]
    },
    {
      "id": "cutover",
      "name": "Cutover",
      "rows": [
        { "date": "2026-11-08 02:05", "action": "Freeze content and copy data", "detail": "Final sync took 4 minutes." },
        { "date": "2026-11-08 02:12", "action": "MILESTONE: DNS switched to new host", "detail": "TTL was lowered to 300 seconds two days earlier." },
        { "date": "2026-11-08 02:20", "action": "EXPOSURE: Debug page reachable for 6 minutes", "detail": "Found during checks and disabled at 02:26. No requests to it in the access log." },
        { "date": "2026-11-08 03:00", "action": "Old VM powered off", "detail": "Kept for 14 days before deletion." }
      ]
    }
  ]
}
