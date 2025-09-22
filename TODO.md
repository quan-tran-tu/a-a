- Write tests
    - Write mock test for parser.GeneratePlan
    - Write mock tests for actions (web, system)
    - Write E2E tests

- Prompt need to be a bit stricter for ollama models

#### Use cases:
1. Daily news briefing: \
Fetch today news from a few news site (CNN, BBC,...) and summarize into a daily report
2. Automated job applicant: \
Fetch jobs listed on Upwork,... and return to the user suitable jobs fitting their preferences
3. Discover SOICT teachers
Fetch information of all teachers listed on https://soict.hust.edu.vn/can-bo to help user find whomever match their academic interest
query to test: i want to list information about all teachers listed on 'https://soict.hust.edu.vn/can-bo'. you might need to analyze the html source of the given url because it might have a page navigation pointing to page 2, 3, ... the end result should be the teacher's name, field of research, papers, awards saved to a soict_teacher.json file. give me the plan first
4. Find products reviews
Find reviews about any products
