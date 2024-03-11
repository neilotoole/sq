# drivers/json/testdata


## Cities

The various "cities" JSON files contain an array of data like this:

```json
{
  "name": "Sant Julià de Lòria",
  "lat": "42.46372",
  "lng": "1.49129",
  "country": "AD",
  "admin1": "06",
  "admin2": ""
}
```



- [`cities.large.json`](cities.large.json) is `146,994` cities (~20MB uncompressed).
- [`cities.small.json`](cities.small.json) 4 city objects.
- [`cities.sample-mixed-10.json`](cities.sample-mixed-10.json) contains 10 objects, but the `admin1` field
  of the third object is the string `TEXT` instead of a number.


