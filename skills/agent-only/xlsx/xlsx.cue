changelog: [{
	date:    "2026-09-28"
	version: "1.4"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  4599
}, {
	date:    "2026-08-23"
	version: "1.3"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-07-30"
	version: "1.2"
	text:    "moved under skills/agent/ group; name and behavior unchanged"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section (routing, preconditions, workflow position)"
}, {
	date:    "2026-07-10"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " xlsx"
}, {
	key:   "description"
	value: " \"Use this skill any time a spreadsheet file is the primary input or output. This means any task where the user wants to: open, read, edit, or fix an existing .xlsx, .xlsm, .csv, or .tsv file (e.g., adding columns, computing formulas, formatting, charting, cleaning messy data); create a new spreadsheet from scratch or from other data sources; or convert between tabular file formats. Trigger especially when the user references a spreadsheet file by name or path — even casually (like \\\"the xlsx in my downloads\\\") — and wants something done to it or produced from it. Also trigger for cleaning or restructuring messy tabular data files (malformed rows, misplaced headers, junk data) into proper spreadsheets. The deliverable must be a spreadsheet file. Do NOT trigger when the primary deliverable is a Word document, HTML report, standalone Python script, database pipeline, or Google Sheets API integration, even if tabular data is involved.\""
}, {
	key:   "license"
	value: " Proprietary. LICENSE.txt has complete terms"
}, {
	key:   "author"
	value: " Anthropic"
}, {
	key:   "version"
	value: " 1.4"
}]
sections: [{
	title: "When to use"
	blocks: [{
		prose: """
			**Use when:** a spreadsheet file (.xlsx, .xlsm, .csv, .tsv) is the primary input or output — reading, editing, creating, or
			converting tabular data files. Trigger especially when the user references a spreadsheet file by name or path.
			**Don't use when:** the primary deliverable is a Word document — anthropic-skills:docx. HTML report, standalone Python script,
			database pipeline, or Google Sheets API integration — even if tabular data is involved. The deliverable is a presentation —
			anthropic-skills:pptx.
			**Workflow position:** standalone (see README.md § Skill map, smine repo).

			# Requirements for Outputs
			"""
	}]
}, {
	title: "All Excel files"
	blocks: [{
		heading: "Professional Font"
	}, {
		prose: """
			- Use a consistent, professional font (e.g., Arial, Times New Roman) for all deliverables unless otherwise instructed by the
			  user
			"""
	}, {
		heading: "Zero Formula Errors"
	}, {
		prose: "- Every Excel model MUST be delivered with ZERO formula errors (#REF!, #DIV/0!, #VALUE!, #N/A, #NAME?)"
	}, {
		heading: "Preserve Existing Templates (when updating templates)"
	}, {
		prose: """
			- Study and EXACTLY match existing format, style, and conventions when modifying files
			- Never impose standardized formatting on files with established patterns
			- Existing template conventions ALWAYS override these guidelines
			"""
	}]
}, {
	title: "Financial models"
	blocks: [{
		heading: "Color Coding Standards"
	}, {
		prose: """
			Unless otherwise stated by the user or existing template

			#### Industry-Standard Color Conventions
			- **Blue text (RGB: 0,0,255)**: Hardcoded inputs, and numbers users will change for scenarios
			- **Black text (RGB: 0,0,0)**: ALL formulas and calculations
			- **Green text (RGB: 0,128,0)**: Links pulling from other worksheets within same workbook
			- **Red text (RGB: 255,0,0)**: External links to other files
			- **Yellow background (RGB: 255,255,0)**: Key assumptions needing attention or cells that need to be updated
			"""
	}, {
		heading: "Number Formatting Standards"
	}, {
		prose: """
			#### Required Format Rules
			- **Years**: Format as text strings (e.g., "2024" not "2,024")
			- **Currency**: Use $#,##0 format; ALWAYS specify units in headers ("Revenue ($mm)")
			- **Zeros**: Use number formatting to make all zeros "-", including percentages (e.g., "$#,##0;($#,##0);-")
			- **Percentages**: Default to 0.0% format (one decimal)
			- **Multiples**: Format as 0.0x for valuation multiples (EV/EBITDA, P/E)
			- **Negative numbers**: Use parentheses (123) not minus -123
			"""
	}, {
		heading: "Formula Construction Rules"
	}, {
		prose: """
			#### Assumptions Placement
			- Place ALL assumptions (growth rates, margins, multiples, etc.) in separate assumption cells
			- Use cell references instead of hardcoded values in formulas
			- Example: Use =B5*(1+$B$6) instead of =B5*1.05

			#### Formula Error Prevention
			- Verify all cell references are correct
			- Check for off-by-one errors in ranges
			- Ensure consistent formulas across all projection periods
			- Test with edge cases (zero values, negative numbers)
			- Verify no unintended circular references

			#### Documentation Requirements for Hardcodes
			- Comment or in cells beside (if end of table). Format: "Source: [System/Document], [Date], [Specific Reference], [URL if
			  applicable]"
			- Examples:
			  - "Source: Company 10-K, FY2024, Page 45, Revenue Note, [SEC EDGAR URL]"
			  - "Source: Company 10-Q, Q2 2025, Exhibit 99.1, [SEC EDGAR URL]"
			  - "Source: Bloomberg Terminal, 8/15/2025, AAPL US Equity"
			  - "Source: FactSet, 8/20/2025, Consensus Estimates Screen"

			# XLSX creation, editing, and analysis
			"""
	}]
}, {
	title: "Overview"
	blocks: [{
		prose: """
			A user may ask you to create, edit, or analyze the contents of an .xlsx file. You have different tools and workflows available
			for different tasks.
			"""
	}]
}, {
	title: "Important Requirements"
	blocks: [{
		prose: """
			**LibreOffice Required for Formula Recalculation**: You can assume LibreOffice is installed for recalculating formula values
			using the `scripts/recalc.py` script. The script automatically configures LibreOffice on first run, including in sandboxed
			environments where Unix sockets are restricted (handled by `scripts/office/soffice.py`)
			"""
	}]
}, {
	title: "Reading and analyzing data"
	blocks: [{
		heading: "Data analysis with pandas"
	}, {
		prose: """
			For data analysis, visualization, and basic operations, use **pandas** which provides powerful data manipulation capabilities:

			```python
			import pandas as pd

			# Read Excel
			df = pd.read_excel('file.xlsx')  # Default: first sheet
			all_sheets = pd.read_excel('file.xlsx', sheet_name=None)  # All sheets as dict

			# Analyze
			df.head()      # Preview data
			df.info()      # Column info
			df.describe()  # Statistics

			# Write Excel
			df.to_excel('output.xlsx', index=False)
			```
			"""
	}]
}, {
	title: "Excel File Workflows"
}, {
	title: "CRITICAL: Use Formulas, Not Hardcoded Values"
	blocks: [{
		prose: """
			**Always use Excel formulas instead of calculating values in Python and hardcoding them.** This ensures the spreadsheet remains
			dynamic and updateable.
			"""
	}, {
		heading: "❌ WRONG - Hardcoding Calculated Values"
	}, {
		prose: """
			```python
			# Bad: Calculating in Python and hardcoding result
			total = df['Sales'].sum()
			sheet['B10'] = total  # Hardcodes 5000

			# Bad: Computing growth rate in Python
			growth = (df.iloc[-1]['Revenue'] - df.iloc[0]['Revenue']) / df.iloc[0]['Revenue']
			sheet['C5'] = growth  # Hardcodes 0.15

			# Bad: Python calculation for average
			avg = sum(values) / len(values)
			sheet['D20'] = avg  # Hardcodes 42.5
			```
			"""
	}, {
		heading: "✅ CORRECT - Using Excel Formulas"
	}, {
		prose: """
			```python
			# Good: Let Excel calculate the sum
			sheet['B10'] = '=SUM(B2:B9)'

			# Good: Growth rate as Excel formula
			sheet['C5'] = '=(C4-C2)/C2'

			# Good: Average using Excel function
			sheet['D20'] = '=AVERAGE(D2:D19)'
			```

			This applies to ALL calculations - totals, percentages, ratios, differences, etc. The spreadsheet should be able to recalculate
			when source data changes.
			"""
	}]
}, {
	title: "Common Workflow"
	blocks: [{
		prose: """
			1. **Choose tool**: pandas for data, openpyxl for formulas/formatting
			2. **Create/Load**: Create new workbook or load existing file
			3. **Modify**: Add/edit data, formulas, and formatting
			4. **Save**: Write to file
			5. **Recalculate formulas (MANDATORY IF USING FORMULAS)**: Use the scripts/recalc.py script
			   ```bash
			   python scripts/recalc.py output.xlsx
			   ```
			6. **Verify and fix any errors**: 
			   - The script returns JSON with error details
			   - If `status` is `errors_found`, check `error_summary` for specific error types and locations
			   - Fix the identified errors and recalculate again
			   - Common errors to fix:
			     - `#REF!`: Invalid cell references
			     - `#DIV/0!`: Division by zero
			     - `#VALUE!`: Wrong data type in formula
			     - `#NAME?`: Unrecognized formula name
			"""
	}, {
		heading: "Creating new Excel files"
	}, {
		prose: """
			```python
			# Using openpyxl for formulas and formatting
			from openpyxl import Workbook
			from openpyxl.styles import Font, PatternFill, Alignment

			wb = Workbook()
			sheet = wb.active

			# Add data
			sheet['A1'] = 'Hello'
			sheet['B1'] = 'World'
			sheet.append(['Row', 'of', 'data'])

			# Add formula
			sheet['B2'] = '=SUM(A1:A10)'

			# Formatting
			sheet['A1'].font = Font(bold=True, color='FF0000')
			sheet['A1'].fill = PatternFill('solid', start_color='FFFF00')
			sheet['A1'].alignment = Alignment(horizontal='center')

			# Column width
			sheet.column_dimensions['A'].width = 20

			wb.save('output.xlsx')
			```
			"""
	}, {
		heading: "Editing existing Excel files"
	}, {
		prose: """
			```python
			# Using openpyxl to preserve formulas and formatting
			from openpyxl import load_workbook

			# Load existing file
			wb = load_workbook('existing.xlsx')
			sheet = wb.active  # or wb['SheetName'] for specific sheet

			# Working with multiple sheets
			for sheet_name in wb.sheetnames:
			    sheet = wb[sheet_name]
			    print(f"Sheet: {sheet_name}")

			# Modify cells
			sheet['A1'] = 'New Value'
			sheet.insert_rows(2)  # Insert row at position 2
			sheet.delete_cols(3)  # Delete column 3

			# Add new sheet
			new_sheet = wb.create_sheet('NewSheet')
			new_sheet['A1'] = 'Data'

			wb.save('modified.xlsx')
			```
			"""
	}]
}, {
	title: "Recalculating formulas"
	blocks: [{
		prose: """
			Excel files created or modified by openpyxl contain formulas as strings but not calculated values. Use the provided
			`scripts/recalc.py` script to recalculate formulas:

			```bash
			python scripts/recalc.py <excel_file> [timeout_seconds]
			```

			Example:
			```bash
			python scripts/recalc.py output.xlsx 30
			```

			The script:
			- Automatically sets up LibreOffice macro on first run
			- Recalculates all formulas in all sheets
			- Scans ALL cells for Excel errors (#REF!, #DIV/0!, etc.)
			- Returns JSON with detailed error locations and counts
			- Works on both Linux and macOS
			"""
	}]
}, {
	title: "Formula Verification Checklist"
	blocks: [{
		prose: "Quick checks to ensure formulas work correctly:"
	}, {
		heading: "Essential Verification"
	}, {
		prose: """
			- [ ] **Test 2-3 sample references**: Verify they pull correct values before building full model
			- [ ] **Column mapping**: Confirm Excel columns match (e.g., column 64 = BL, not BK)
			- [ ] **Row offset**: Remember Excel rows are 1-indexed (DataFrame row 5 = Excel row 6)
			"""
	}, {
		heading: "Common Pitfalls"
	}, {
		prose: """
			- [ ] **NaN handling**: Check for null values with `pd.notna()`
			- [ ] **Far-right columns**: FY data often in columns 50+ 
			- [ ] **Multiple matches**: Search all occurrences, not just first
			- [ ] **Division by zero**: Check denominators before using `/` in formulas (#DIV/0!)
			- [ ] **Wrong references**: Verify all cell references point to intended cells (#REF!)
			- [ ] **Cross-sheet references**: Use correct format (Sheet1!A1) for linking sheets
			"""
	}, {
		heading: "Formula Testing Strategy"
	}, {
		prose: """
			- [ ] **Start small**: Test formulas on 2-3 cells before applying broadly
			- [ ] **Verify dependencies**: Check all cells referenced in formulas exist
			- [ ] **Test edge cases**: Include zero, negative, and very large values
			"""
	}, {
		heading: "Interpreting scripts/recalc.py Output"
	}, {
		prose: """
			The script returns JSON with error details:
			```json
			{
			  "status": "success",           // or "errors_found"
			  "total_errors": 0,              // Total error count
			  "total_formulas": 42,           // Number of formulas in file
			  "error_summary": {              // Only present if errors found
			    "#REF!": {
			      "count": 2,
			      "locations": ["Sheet1!B5", "Sheet1!C10"]
			    }
			  }
			}
			```
			"""
	}]
}, {
	title: "Best Practices"
	blocks: [{
		heading: "Library Selection"
	}, {
		prose: """
			- **pandas**: Best for data analysis, bulk operations, and simple data export
			- **openpyxl**: Best for complex formatting, formulas, and Excel-specific features
			"""
	}, {
		heading: "Working with openpyxl"
	}, {
		prose: """
			- Cell indices are 1-based (row=1, column=1 refers to cell A1)
			- Use `data_only=True` to read calculated values: `load_workbook('file.xlsx', data_only=True)`
			- **Warning**: If opened with `data_only=True` and saved, formulas are replaced with values and permanently lost
			- For large files: Use `read_only=True` for reading or `write_only=True` for writing
			- Formulas are preserved but not evaluated - use scripts/recalc.py to update values
			"""
	}, {
		heading: "Working with pandas"
	}, {
		prose: """
			- Specify data types to avoid inference issues: `pd.read_excel('file.xlsx', dtype={'id': str})`
			- For large files, read specific columns: `pd.read_excel('file.xlsx', usecols=['A', 'C', 'E'])`
			- Handle dates properly: `pd.read_excel('file.xlsx', parse_dates=['date_column'])`
			"""
	}]
}, {
	title: "Code Style Guidelines"
	blocks: [{
		prose: """
			**IMPORTANT**: When generating Python code for Excel operations:
			- Write minimal, concise Python code without unnecessary comments
			- Avoid verbose variable names and redundant operations
			- Avoid unnecessary print statements

			**For Excel files themselves**:
			- Add comments to cells with complex formulas or important assumptions
			- Document data sources for hardcoded values
			- Include notes for key calculations and model sections
			"""
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "script-driven spreadsheet work via openpyxl recipes"
			tier:   "mid-tier"
		}
	}]
}]
smoke: {
	skip: "its spreadsheet steps run python3, which the settings deny list blocks"
}
